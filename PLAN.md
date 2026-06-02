# Build Plan

Each step will have its own spec before implementation starts.

---

## Phase 1 — Repo & Scaffolding

**Step 1: Project structure & tooling**
- Create `backend/` and `frontend/` directories
- Go module init, `frontend/` Vite + pnpm + TypeScript setup
- Makefile with `dev`, `build`, `test`, `seed` targets
- Verify clean-clone startup works end to end

---

## Phase 2 — Backend Foundation

**Step 2: Config, logging, error handling**
- Load all env vars at startup, fail fast if required vars are missing
- Structured JSON logger with sane levels
- Correlation ID middleware (generate + attach to every request/response)
- Centralized error handler — typed error body, 4xx vs 5xx, no stack traces

**Step 3: Data layer (SQLite)**
- Open `predictions.db` read-only
- Queries: list photos (cursor pagination + filters), get one photo, get predictions for a photo
- Filter logic: single prediction must satisfy both `classId` AND `minConfidence`

**Step 4: MinIO integration**
- MinIO client setup from env vars
- Presigned PUT URL generation (upload-link)
- Presigned GET URL generation (1-hour TTL, fresh per API response)

**Step 5: API routes**
- `POST /photos/{photoId}/upload-link`
- `GET /photos` with cursor pagination and filters
- `GET /photos/{photoId}`
- All routes validated against `openapi.yaml` contract

---

## Phase 3 — Thumbnail Engine

**Step 6: Thumbnail endpoint**
- `GET /thumbnails/{photoId}?w={width}&dpr={dpr}&fmt={fmt}`
- On-demand JPEG → WebP/JPEG resize at `w × dpr` pixels
- Singleflight coalescing on `(photoId, w, dpr, fmt)`
- Semaphore: max 4 concurrent generations; 503 Retry-After when full
- Disk LRU cache, 500 MB default cap (`THUMB_CACHE_SIZE_MB`)

---

## Phase 4 — Metrics

**Step 7: `/metrics` endpoint**
- Request rate, latency (p50/p95), error rate
- Thumbnail cache hit/miss count, generation time histogram

---

## Phase 5 — Seed Script

**Step 8: Seed**
- Iterate `dataset/images/*.jpg`
- POST `{ contentType: "image/jpeg" }` → get presigned PUT URL + headers
- PUT file bytes with forwarded headers
- Idempotent (skip if already uploaded)

---

## Phase 6 — Frontend Foundation

**Step 9: Frontend setup**
- Vite + React 19 + TypeScript + pnpm
- Feature-based folder structure: `features/gallery/`, `features/filters/`, `features/map/`
- `openapi-typescript` codegen from `openapi.yaml` — add to build scripts
- Redux Toolkit store: filters slice, selected photo slice

**Step 10: API client**
- Typed fetch wrapper using generated types
- Request interceptor: attach `X-API-Key` header
- Loading / error / empty state handling at the data layer

---

## Phase 7 — Gallery

**Step 11: Gallery grid**
- Scrolling paginated grid
- Thumbnails via `srcset`/`sizes` using the thumbnail endpoint
- Infinite scroll or pagination controls
- Loading, empty, and error states

**Step 12: Bbox overlay**
- Canvas or SVG overlay on each thumbnail
- Normalized bbox → rendered CSS pixel coordinates: `bbox_x * renderedWidth`
- Correct at every size and DPR
- Overlay updates on image resize (ResizeObserver)

**Step 13: Filters**
- Class filter (powdery_mildew, mirid, whitefly_aphid, miner_tuta, thrips, spider_mites)
- Min confidence slider
- State in Redux — shared with map view
- Drives `GET /photos` query params

**Step 14: Photo modal**
- Click thumbnail → full-size photo with all predictions overlaid
- Bbox overlay at full resolution
- Prediction list (class, confidence) alongside image

---

## Phase 8 — Greenhouse Map (Bonus)

**Step 15: Map view**
- Konva canvas, 40×40 m floor plan
- Each photo placed at its `x,y` position
- Zoom and pan
- Click a location → filter gallery to nearby photos (user-defined radius)
- Class filter drives both map and gallery (shared Redux state)

---

## Phase 9 — Tests

**Step 16: Required tests**
1. bbox coordinate transform: normalized [0,1] → rendered px at arbitrary size/DPR
2. Thumbnail request parse/validate + `w × dpr` math
3. Filter reducer (Redux) or a key gallery component
4. Backend smoke: seed one photo → `GET /photos/{photoId}` returns it with correct fields

---

## Phase 10 — Polish & Verification

**Step 17: Clean-clone verification**
- Follow the README setup steps on a clean environment
- Confirm: `docker compose up` → seed → backend → frontend all work
- All tests pass
- No blank screens, no broken images under any filter combination