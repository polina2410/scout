# Architecture Decisions

Decisions made to fill gaps in the assignment spec. Each was identified by a pre-implementation critique and resolved before coding began.

---

## 5. TypeScript pinned to 5.9.3; vitest pinned to 4.1.7

**Problem:** `pnpm create vite` scaffolded TypeScript 6.0.3. `openapi-typescript@7.13.0` (latest) declares `peerDependencies: { typescript: "^5.x" }` and does not support TypeScript 6 yet. `vitest@4.1.8` was published the day of setup (2026-06-01), failing the one-week-old rule.

**Decision:** Pin `typescript@5.9.3` (latest 5.x, published 2025-09-30) and `vitest@4.1.7` (published 2026-05-20).

**Why:** `openapi-typescript` is a core build tool — type generation breaking at install would block all frontend API work. The downgrade is a no-op functionally; TypeScript 5.9 is stable. `vitest@4.1.7` is one patch behind latest with no known regressions; upgrade when `4.1.8` is older than a week.

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

## 6. Logger package location

**Problem:** `NewLogger` needs to live somewhere. Options: `internal/middleware` (first consumer), `internal/logger` (dedicated package), or `main` (inline setup).

**Decision:** Dedicated `internal/logger` package with a single `New(w io.Writer, level string) *slog.Logger` function.

**Why:** `internal/middleware` importing a logger it also constructs creates a subtle coupling — the package does two unrelated things. Inlining in `main` makes the function untestable. A dedicated `internal/logger` package is one file, zero dependencies beyond `log/slog`, and can be imported by middleware, handlers, and any future package without circular imports.

---

## 4. Seed script `Content-Type` handling

**Problem:** The `POST /photos/{photoId}/upload-link` spec requires `{ "contentType": "image/jpeg" }` in the request body, and the response includes a `headers` map to forward onto the PUT. CLAUDE.md's original seed description omitted both. An implementer following it would get a 400 from their own validation, or store objects as `application/octet-stream`, breaking browser image rendering.

**Decision:** Seed script must:
1. Send `{ "contentType": "image/jpeg" }` in the POST body
2. Forward every key in `UploadLink.headers` as HTTP headers on the PUT to MinIO

**Why:** This is what the spec already requires — not a new decision, just a gap that needed to be made explicit. The `headers` map exists precisely to allow the backend to attach storage-provider-specific headers (e.g., `Content-Type`, `x-amz-*`) without the client needing to know which provider is behind the presigned URL.