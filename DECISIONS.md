# Architecture Decisions

Decisions made to fill gaps in the assignment spec. Each was identified by a pre-implementation critique and resolved before coding began.

---

## 1. Thumbnail URL interface

**Problem:** The README says "the interface is yours" but the gallery's bbox math depends on knowing the exact rendered pixel dimensions, which are determined by the thumbnail URL shape. Without an agreed contract, backend and frontend would conflict.

**Decision:** `GET /thumbnails/{photoId}?w={width}&dpr={dpr}&fmt={fmt}`

- `w` = CSS pixel width (required)
- `dpr` = 1 | 2 | 3, default 1
- `fmt` = `webp` | `jpeg`, default `webp`
- Server generates at `w × dpr` actual pixels

**Why:** Query params are the most cache-friendly shape (CDN/proxy cache on full URL), easy to construct `srcset` strings from, and human-readable. The `w` param maps directly to CSS pixels, keeping bbox math simple: `bbox_x * w` gives the pixel offset in the rendered image.

---

## 2. `originalUrl` presigning strategy

**Problem:** The spec says `originalUrl` "may be presigned/time-limited" but doesn't define the TTL, whether it's regenerated per-response or stored, or what happens when it expires. A stale presigned URL causes broken images in the gallery — violating the "never a blank screen" requirement.

**Decision:** Generate a fresh presigned GET URL (1-hour TTL) for every photo on every API response. Never store the URL in the database.

**Why:** A 1-hour TTL is long enough to survive any reasonable user session. Regenerating per-response keeps the backend stateless (no URL rotation job needed). The tradeoff is that the frontend cannot safely cache photo objects long-term — documented as a constraint. A public bucket would be simpler but less realistic for a production monitoring system.

---

## 3. Thumbnail engine resource bounds

**Problem:** CLAUDE.md said the cache must be "resource-bounded" and "concurrency-safe" but defined no actual limits. Decoding a 2560×1440 JPEG allocates ~15 MB. Ten simultaneous gallery loads for ten different originals = ~150 MB in-flight on a 512 MB server.

**Decision:**
- Max **4 concurrent generations** via semaphore; return `503 Retry-After` when full
- **Singleflight** on `(photoId, w, dpr, fmt)` — in-flight duplicates share one generation
- **Disk LRU cache**, default 500 MB, configurable via `THUMB_CACHE_SIZE_MB`

**Why:** 4 workers allows ~60 MB peak in-flight allocation (well within 512 MB), leaves headroom for the Go runtime and the API server. Singleflight prevents thundering-herd on cold cache. Disk cache (not memory) keeps the memory footprint flat — generated thumbnails are small (10–50 KB WebP) and fast to read from disk vs. re-generating.

---

## 4. Seed script `Content-Type` handling

**Problem:** The `POST /photos/{photoId}/upload-link` spec requires `{ "contentType": "image/jpeg" }` in the request body, and the response includes a `headers` map to forward onto the PUT. CLAUDE.md's original seed description omitted both. An implementer following it would get a 400 from their own validation, or store objects as `application/octet-stream`, breaking browser image rendering.

**Decision:** Seed script must:
1. Send `{ "contentType": "image/jpeg" }` in the POST body
2. Forward every key in `UploadLink.headers` as HTTP headers on the PUT to MinIO

**Why:** This is what the spec already requires — not a new decision, just a gap that needed to be made explicit. The `headers` map exists precisely to allow the backend to attach storage-provider-specific headers (e.g., `Content-Type`, `x-amz-*`) without the client needing to know which provider is behind the presigned URL.

---

## 5. TypeScript pinned to 5.9.3; vitest pinned to 4.1.7

**Problem:** `pnpm create vite` scaffolded TypeScript 6.0.3. `openapi-typescript@7.13.0` (latest) declares `peerDependencies: { typescript: "^5.x" }` and does not support TypeScript 6 yet. `vitest@4.1.8` was published the day of setup (2026-06-01), failing the one-week-old rule.

**Decision:** Pin `typescript@5.9.3` (latest 5.x, published 2025-09-30) and `vitest@4.1.7` (published 2026-05-20).

**Why:** `openapi-typescript` is a core build tool — type generation breaking at install would block all frontend API work. The downgrade is a no-op functionally; TypeScript 5.9 is stable. `vitest@4.1.7` is one patch behind latest with no known regressions; upgrade when `4.1.8` is older than a week.

---

## 6. Logger package location

**Problem:** `NewLogger` needs to live somewhere. Options: `internal/middleware` (first consumer), `internal/logger` (dedicated package), or `main` (inline setup).

**Decision:** Dedicated `internal/logger` package with a single `New(w io.Writer, level string) *slog.Logger` function.

**Why:** `internal/middleware` importing a logger it also constructs creates a subtle coupling — the package does two unrelated things. Inlining in `main` makes the function untestable. A dedicated `internal/logger` package is one file, zero dependencies beyond `log/slog`, and can be imported by middleware, handlers, and any future package without circular imports.

---

## 7. Thumbnail cache: atomic writes and key allowlist

**Problem:** Two concurrent requests for the same cache key could interleave partial writes to the same file. A path-traversal key (e.g. containing `..` or `/`) could escape the cache directory. Both must be safe regardless of the singleflight layer above.

**Decision:**
- `Put` writes to a `os.CreateTemp` file with a `.tmp-` prefix in the same cache directory, then calls `os.Rename` into the final path. The last rename wins; partial files are never visible to `Get`.
- `safePath` rejects any key that does not match `^[A-Za-z0-9_-]+$` before constructing the file path. The allowlist covers every character the key format `{photoId}_{w}_{dpr}_{fmt}` can produce and nothing else.

**Why:** Atomic rename is the standard POSIX idiom for safe concurrent file writes — it is guaranteed to be atomic on the same filesystem. The allowlist approach is stricter than a denylist: it does not depend on knowing all dangerous characters or filesystem case-sensitivity rules. Singleflight reduces concurrent `Put` calls to near-zero in practice, but the cache must be correct if a caller bypasses it.

Source: `backend/internal/thumb/cache.go` — `Put` lines 159–192, `safePath` lines 118–123, `validCacheKey` line 17.

---

## 8. `presignAll` cancellable context

**Problem:** `GET /photos` fans out up to 10 concurrent MinIO presign calls. If one fails (or the client disconnects), the remaining goroutines should not keep running indefinitely.

**Decision:** `presignAll` derives a child `context.WithCancel` from the request context. On the first error it calls `cancel()`, which causes goroutines blocked on `sem <- struct{}{}` to return immediately via the `ctx.Done()` select arm. Goroutines already executing a MinIO SDK call run to completion (the SDK does not support mid-call abort), but their results are discarded. The results channel is buffered to `len(photos)` so the closer goroutine (`wg.Wait(); close(results)`) never blocks.

**Why:** This avoids goroutine leaks without a separate cleanup mechanism. The tradeoff — that in-flight MinIO calls complete even after cancellation — is acceptable because presigning is a cheap metadata operation, not a data transfer.

Source: `backend/internal/handler/photos.go` — `presignAll` lines 96–150.

---

## 9. Rate limiter proxy-trust is opt-in

**Problem:** The thumbnail endpoint is unauthenticated. The per-IP rate limiter needs a reliable client IP. When running behind a reverse proxy, `RemoteAddr` is the proxy's IP — every client collapses into one bucket. But trusting `X-Forwarded-For` when not behind a proxy lets any client spoof a different IP and evade the limit.

**Decision:** `TRUST_PROXY_HEADERS=false` by default. Operators set it to `true` only when the server is known to be behind a trusted reverse proxy. When `true`, the rate limiter reads the left-most `X-Forwarded-For` entry, falling back to `X-Real-IP`, then `RemoteAddr`.

**Why:** Defaulting to safe (distrust headers) means a misconfigured deployment fails closed — all traffic is rate-limited per proxy IP rather than per client, which is suboptimal but not a security hole. Defaulting to trust would be the opposite: a direct-exposure deployment silently becomes bypassable.

Source: `backend/internal/config/config.go` lines 88–93; `backend/internal/middleware/ratelimit.go` — `clientIP` lines 105–123.

---

## 10. Frontend data hooks use AbortController for cancellation

**Problem:** React strict mode double-invokes effects; filter changes and component unmounts can leave stale fetch callbacks that dispatch into a reset or unmounted reducer.

**Decision:** All three data hooks (`usePhotos`, `usePhotoDetail`, `useAllPhotos`) create an `AbortController` inside `useEffect`, pass `controller.signal` to the fetch call, and abort on cleanup. Dispatch is guarded by `controller.signal.aborted` checks so stale responses are silently dropped, not applied.

Additional constraints per hook:
- `usePhotos` also stores a `loadMoreControllerRef` to abort an in-flight "load more" request when filters change before it completes.
- `useAllPhotos` (map view) caps the fetch loop at 20 pages × 50 photos (`MAX_PAGES = 20`, `PAGE_SIZE = 50`) to bound memory and request count for the eager full-dataset load.

**Why:** `AbortController` is the standard browser API for cancelling fetch requests. Guarding dispatch on `signal.aborted` (rather than a separate `cancelled` ref) uses the same flag the network layer already sets, avoiding a second source of truth.

Source: `frontend/src/features/gallery/usePhotos.ts`, `frontend/src/features/gallery/usePhotoDetail.ts`, `frontend/src/features/map/useAllPhotos.ts`.

---

## 11. Go toolchain pinned to go1.26.4

**Problem:** The Go module minimum (`go 1.25.0` in `go.mod`) is the lowest compatible version, but the actual build toolchain used in development was updated to patch security vulnerabilities in the standard library.

**Decision:** `go.mod` carries `toolchain go1.26.4`. Indirect dependencies `golang.org/x/net`, `golang.org/x/crypto`, and `golang.org/x/sys` were bumped at the same time to their current versions.

**Why:** The `toolchain` directive (Go 1.21+) separates the minimum-compatibility floor from the actual build toolchain. Pinning it in `go.mod` means `go toolchain` enforcement will refuse to build with an older toolchain, making the security constraint explicit and reproducible across CI and developer machines without requiring a wrapper script.

Source: `backend/go.mod` lines 5–6, 32–34.

---

## 12. Frontend supply-chain guard via `pnpm-workspace.yaml`

**Problem:** CLAUDE.md requires that only dependencies published for at least one week may be installed or upgraded. Enforcing this by code review alone is error-prone.

**Decision:** `frontend/pnpm-workspace.yaml` sets `minimumReleaseAge: 10080` (minutes = 7 days). pnpm 10+ refuses to install any package version published less than that many minutes ago, making the rule a hard install-time check rather than a guideline.

**Why:** Automating the check catches transitive upgrades, not just direct ones. 10080 minutes is exactly the one-week threshold from CLAUDE.md. The field has no effect on pnpm < 10, so it is safe to commit without breaking older toolchains.

Source: `frontend/pnpm-workspace.yaml` line 4.
