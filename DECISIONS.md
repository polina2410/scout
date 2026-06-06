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

---

## 13. UI is dark-only; theme pinned at the document root

**Problem:** `index.css` declared `color-scheme: light dark` with a white `:root` background and applied the dark palette only inside a `@media (prefers-color-scheme: dark)` block, while every component hard-codes dark colors. On a light-mode OS the page background fell back to white: a white flash before React mounts, and a fully white gallery panel whenever the grid was empty (the `.main` element had no background of its own, so the white document showed through). A live Playwright review confirmed both.

**Decision:** Treat Scout as a dark-only UI. `:root` is now `color-scheme: dark` with `background: #16171d` unconditionally (no `prefers-color-scheme` branch), and the gallery `.main` carries its own `#16171d` background as defense-in-depth.

**Why:** All component styling is already dark; supporting a real light theme would mean re-theming every module. Pinning the scheme at the root is the smallest change that removes the flash and the empty-state white panel for every OS preference, and `color-scheme: dark` also makes native controls/scrollbars render dark. The redundant `.main` background guarantees the empty/loading states never expose the document background even if the root rule changes.

Source: `frontend/src/index.css` lines 5–13; `frontend/src/App.module.css` `.main`.

---

## 14. Bounding-box legibility via a dark halo, not per-class recoloring

**Problem:** `CLASS_COLORS.powdery_mildew` is `#ffffff`. A 2px white stroke is legible on dark foliage but washes out on pale/blown-out regions of a photo — and powdery mildew is the most common class. The naive fix (recolor `powdery_mildew`) was flagged by a static review as "Critical/invisible."

**Decision:** Keep `CLASS_COLORS` unchanged. In `BboxCanvas`, draw each box twice — a darker, wider underlay (`rgba(0,0,0,0.55)`, 4px) first, then the class color (2px) on top — so every box gets a contrast halo on any background.

**Why:** The halo fixes legibility for *all* classes on *any* background, not just the white one, while preserving the agreed class palette. Recoloring only `powdery_mildew` would leave the same risk for other light colors (e.g. `whitefly_aphid` yellow on a bright leaf) and would also change the map dots, which reuse `CLASS_COLORS` against a dark floor where white reads fine. Line widths are extracted to named constants (`BOX_LINE_WIDTH`, `BOX_HALO_WIDTH`) per the no-magic-numbers rule.

Source: `frontend/src/features/gallery/BboxCanvas.tsx`.

---

## 15. Disabled-control hints and empty states are recoverable in-place

**Problem:** Two states left the user without a visible next step. The map radius slider is `disabled` until a location is set, but its explanatory hint lived in a screen-reader-only span — sighted users saw a greyed slider with no reason. And the empty gallery showed only "No photos found.", with the only recovery (Reset) far away in the top filter bar.

**Decision:**
- The radius hint is a visible caption (`styles.hint`) rendered below the controls when no location filter is active; it still serves as the slider's `aria-describedby` target, so one element covers both sighted and assistive-tech users.
- The empty state renders an inline **"Clear filters"** button — shown only when a filter is actually active (`classId`, `minConfidence > 0`, or `locationFilter`) — that dispatches `resetFilters`.

**Why:** A hint that explains a disabled control belongs on screen, not only in the accessibility tree; collapsing the visible and `aria-describedby` text into one node avoids duplicated, drifting copy. Gating the Clear-filters button on an active filter avoids offering a no-op action when the dataset is genuinely empty.

Source: `frontend/src/features/map/MapView.tsx`; `frontend/src/features/gallery/GalleryGrid.tsx`.

---

## 16. Photo ordering and cursor encoding

**Problem:** The contract specifies cursor pagination with an "opaque" token and a 1–200 / default-50 limit, but never defines the *order* photos come back in. Without a stable, total order, keyset pagination can skip or repeat rows, and "newest first" vs. "by position" is a product choice.

**Decision:** Order by `captured_at DESC, id DESC` (newest photo first; `id` breaks ties for a total order). The cursor is `base64url("<RFC3339 captured_at>|<id>")`, and the page query fetches `limit + 1` rows to detect a next page without a separate `COUNT`.

**Why:** Newest-first is the useful default for a monitoring feed. A composite `(captured_at, id)` keyset is stable under inserts and far cheaper than `OFFSET` paging on a growing table. Encoding both fields in the cursor lets the `WHERE (captured_at < ? OR (captured_at = ? AND id < ?))` clause resume deterministically even when many photos share a timestamp. The `+1` lookahead avoids a second round-trip just to know whether `next_token` should be set.

Source: `backend/internal/db/db.go` — `ListPhotos` ordering lines 117–138, cursor decode lines 97–112, encode lines 161–167.

---

## 17. SQLite access: pure-Go driver, read-only DSN, single connection

**Problem:** CLAUDE.md says `predictions.db` is a read-only source of truth and forbids an ORM, but does not pick a driver or a concurrency model. The common `mattn/go-sqlite3` driver requires cgo, which complicates cross-compilation and static builds.

**Decision:** Use the pure-Go `modernc.org/sqlite` driver. Open with DSN `file:<path>?mode=ro&_busy_timeout=5000`, and cap the pool at `SetMaxOpenConns(1)` / `SetMaxIdleConns(1)`.

**Why:** A pure-Go driver keeps the binary cgo-free and portable (and is what the thumbnail engine's no-cgo build path also relies on — see decision #18). `mode=ro` enforces the read-only invariant at the driver level, so a stray write fails loudly rather than mutating the provided DB. A single connection serializes access to one file handle: the dataset is tiny and read-only, so there is no concurrency benefit to a larger pool, and one connection sidesteps SQLite's writer-lock and busy-retry edge cases entirely. `_busy_timeout` is belt-and-suspenders for the rare lock contention.

Source: `backend/internal/db/db.go` — `Open` lines 42–56, `NewDB` lines 36–40.

---

## 18. WebP encoding via cgo build tag, with JPEG fallback

**Problem:** The thumbnail design prefers WebP (see decision #1), but Go has no WebP *encoder* in its standard library or in `golang.org/x/image`. The good encoders bind libwebp through cgo, which conflicts with the cgo-free build goal and would break any environment without a C toolchain.

**Decision:** Split encoding across build tags. `encode_cgo.go` (`//go:build cgo`) encodes real WebP via `github.com/chai2010/webp`; `encode_nocgo.go` (`//go:build !cgo`) encodes JPEG only. In the no-cgo build, `effectiveFormat("webp")` returns `"jpeg"`, and that effective format is what gets used for both the cache key (`{photoId}_{w}_{dpr}_{fmt}`) and the response `Content-Type`.

**Why:** This lets the same codebase produce true WebP where a C toolchain exists and degrade gracefully to JPEG where it doesn't, without the frontend or cache needing to know which build is running. Keying the cache on the *effective* format prevents a future cgo build from serving a JPEG that an earlier no-cgo build cached under a `_webp` key. JPEG at q85 is a perfectly serviceable fallback; the only cost is larger bytes over the wire.

Source: `backend/internal/thumb/encode_cgo.go`, `backend/internal/thumb/encode_nocgo.go`; cache-key use in `service.go` lines 126–129.

---

## 19. Thumbnail resampling and quality

**Problem:** Generating a thumbnail from a 2560×1440 original requires choosing a downscale algorithm and an encoder quality — a direct quality-vs-CPU/size tradeoff that nothing in the spec pins down.

**Decision:** Downscale with `golang.org/x/image/draw.CatmullRom`. Encode JPEG at quality 85 and WebP at quality 80. The output height is derived from the requested width times the source aspect ratio, so thumbnails are never distorted.

**Why:** Catmull-Rom is a high-quality resampling kernel — noticeably sharper than bilinear/approx-bilinear for photographic downscales, and the per-image cost is acceptable under the 4-slot generation cap (see decision #3). q85 JPEG / q80 WebP sit at the usual "visually lossless for thumbnails" sweet spot. Deriving height from the source preserves the bbox coordinate math, which assumes the rendered image keeps the original aspect ratio.

Source: `backend/internal/thumb/service.go` — `generate` lines 192–201; `jpegQuality` line 27, `webpQuality` in `encode_cgo.go` line 13.

---

## 20. Metrics are custom JSON, not Prometheus exposition

**Problem:** CLAUDE.md requires `/metrics` to expose request rate, latency, error rate, and thumbnail cache hit/miss + generation time, but not a wire format. The default assumption would be Prometheus text exposition.

**Decision:** Serve a single nested JSON document (`{ requests: {...}, thumbnails: {...} }`) with derived fields (requests/sec, error rate, cache hit rate, mean/p50/p95). Latency and generation time percentiles come from fixed-bound in-process histograms; rates are derived from an uptime counter.

**Why:** There is no Prometheus/scrape stack in this assignment, and a human-readable JSON blob is directly inspectable with `curl` and trivial to assert in tests. Fixed-bucket histograms give stable p50/p95 without retaining per-request samples or pulling in a metrics library, keeping memory flat and the dependency surface minimal. The format is easy to swap for Prometheus later if a scrape target is ever needed.

Source: `backend/internal/handler/metrics.go`; histogram buckets in `thumb/service.go` lines 30–35, `metrics` package collector.

---

## 21. Thumbnail HTTP caching headers

**Problem:** The disk cache (see decision #3) avoids regenerating thumbnails server-side, but without response cache headers every browser and proxy still re-fetches each thumbnail on every view.

**Decision:** Thumbnail responses set `Cache-Control: public, max-age=3600` and an `X-Cache: HIT|MISS` header reflecting the server-side disk-cache outcome.

**Why:** A 1-hour public TTL lets browsers and any shared proxy serve repeat thumbnail views without touching the origin — thumbnails are immutable for a given `(photoId, w, dpr, fmt)` key, so caching is safe. `public` is acceptable because the thumbnail endpoint is unauthenticated by design (it is out of the data-API contract and rate-limited instead). `X-Cache` makes the server-side hit/miss observable per response, complementing the aggregate counters in `/metrics`.

Source: `backend/internal/thumb/service.go` — `serveImage` lines 226–233.

---

## 22. Singleflight generation runs on a detached context

**Problem:** Identical in-flight thumbnail requests are coalesced via singleflight (see decision #3). If that shared generation used the originating request's context, a single caller disconnecting would cancel the work for *every* waiter sharing the key.

**Decision:** The singleflight function runs `generate(context.Background(), p)` — detached from any one caller's request context. Semaphore acquisition inside it is a non-blocking `select`: when all 4 slots are busy it returns `errAtCapacity` immediately, which every coalesced waiter receives as the same `503 Retry-After: 5`.

**Why:** Detaching the context means the first caller's disconnect cannot poison the result for the others queued behind the same key; the generated bytes still land in the cache for the next request regardless. Non-blocking acquisition makes back-pressure explicit and fast — callers get a 503 to retry rather than piling up on a blocked channel and exhausting goroutines/sockets under load.

Source: `backend/internal/thumb/service.go` — `Handle` singleflight + semaphore lines 138–152.

---

## 23. Out-of-contract error statuses reuse the error envelope

**Problem:** The thumbnail endpoint (outside the openapi contract) needs to express "rate limited" (429) and "at capacity" (503), but the contract's error schema only enumerates codes for 400/401/404/500.

**Decision:** Emit 429 with code `TooManyRequests` and 503 with code `ServiceUnavailable`, using the same `{ request_id, message, code }` envelope as the contract errors. The constants are commented as intentionally outside the openapi enum. (401 and 429 are also emitted as string literals from the `middleware` package, which cannot import `handler` without an import cycle.)

**Why:** Reusing one error shape everywhere keeps clients able to parse any failure uniformly, even for statuses the data-API contract never needed. Confining the contract enum to its documented codes — while letting the non-contract thumbnail path add two more — keeps `openapi.yaml` an accurate description of the *data* API without pretending to cover thumbnail delivery.

Source: `backend/internal/handler/handler.go` — error code constants lines 15–22.

---

## 24. Thumbnail rate limiter: token bucket, defaults 30/sec burst 60

**Problem:** The thumbnail endpoint is unauthenticated and backed by a 4-slot generation semaphore. Without rate limiting, a single client could flood it and starve everyone. decision #9 covers *which IP* to attribute a request to, but not the limiter's existence or tuning.

**Decision:** A per-IP token-bucket limiter guards `/thumbnails`, configurable via `THUMB_RATE_PER_SEC` (default 30) and `THUMB_RATE_BURST` (default 60); both must be positive or startup fails. On rejection it returns 429 (see decision #23).

**Why:** A burst of 60 comfortably covers one gallery screen's worth of thumbnail requests loading at once (grid × DPR variants), so normal browsing is never throttled. The 30/sec sustained refill caps a flood at a rate the 4-slot generator can actually keep up with, so the limiter sheds load *before* the semaphore saturates and starts returning 503s. Making both values env-tunable lets operators adjust for their own gallery sizes and hardware.

Source: `backend/internal/config/config.go` lines 72–86; `backend/internal/middleware/ratelimit.go`.

---

## 25. `GET /health` endpoint

**Problem:** Nothing in the contract or CLAUDE.md defines a liveness/readiness probe, but container orchestration, `docker compose` healthchecks, and the README verification flow all benefit from a cheap, unauthenticated "is it up?" signal.

**Decision:** Add `GET /health` returning `{ "status": ..., "version": ... }` as JSON, outside the openapi data-API contract and not requiring `X-API-Key`.

**Why:** A dedicated health route gives deploy tooling and the verification script a dependency-free check that doesn't need an API key or touch SQLite/MinIO. Keeping it out of `openapi.yaml` is deliberate — the contract describes the data API, and operational endpoints (`/health`, `/metrics`, `/thumbnails`) live alongside it rather than inside it.

Source: `backend/internal/handler/handler.go` — `HealthResponse` lines 60–64; route wiring in `backend/cmd/server/main.go`.

---

## 26. Frontend requests thumbnails as JPEG only

**Problem:** The thumbnail engine prefers WebP (see decision #1 and decision #18), but the frontend has to choose a concrete `fmt` for every gallery request and `srcset` entry.

**Decision:** `thumbnailUrl` hard-codes `fmt=jpeg` (with 1×/2×/3× DPR variants in the `srcset`). WebP negotiation is deferred rather than attempted client-side.

**Why:** The default local/dev build runs the server without cgo, where a `webp` request transparently degrades to JPEG anyway (see decision #18) — so requesting `webp` would yield identical bytes under a different cache key, with no benefit and a risk of confusion. Pinning JPEG keeps the delivered format predictable across build configurations. Revisiting this (e.g. `<picture>`/`Accept`-based negotiation to actually ship smaller WebP in cgo builds) is a known follow-up, noted here so the divergence from the "WebP preferred" design is intentional and visible.

Source: `frontend/src/features/gallery/thumbnailUrl.ts`.
