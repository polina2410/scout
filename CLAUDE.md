# Scout — CLAUDE.md

Build the full Scout service from scratch: a Go backend + React/TypeScript frontend for greenhouse pest and disease monitoring. Cameras produce 2560×1440 JPEGs; a CV model annotates them with bounding boxes; growers use Scout to browse detections and locate problems on a map.

## Project layout (to be built)

```
scout-takehome/
├── backend/          Go service (API + thumbnail engine + MinIO seeder)
├── frontend/         React + Vite app
├── dataset/
│   ├── images/       50 JPEGs — ingest into MinIO, do not serve from here
│   └── predictions.db  SQLite — read directly, do not copy or migrate
├── openapi.yaml      Data API contract (source of truth)
├── .env.example
├── .env.local        gitignored — local dev values
├── DECISIONS.md      architecture decisions and spec gap resolutions
└── CLAUDE.md         this file
```

## Stack

**Backend:** Go. SQLite via `predictions.db` (read-only). MinIO for object storage. No ORM.

**Frontend:** React 19, TypeScript, Vite, pnpm, Redux Toolkit, openapi-typescript, react-konva, CSS Modules, Vitest.

**Rules (non-negotiable):**
- `interface` over `type`
- Always use TypeScript — no `any` types, use `unknown` or proper generics
- No magic numbers — extract named constants
- Styles via CSS Modules (`.module.css`) — no inline styles, no global class strings
- Feature-based folder structure in the frontend
- Only add dependencies that have been published for at least one week (check npm/pkg.go.dev publish date before installing)

## Architecture
- One component per file
- No prop drilling beyond 2 levels — use context or lift state
- 
## Data model

```sql
photos(id, x, y, h, width, height, captured_at)
-- x, y: position on 40×40 m greenhouse floor (meters)
-- h: camera height above floor (meters)
-- width, height: original image dimensions in pixels (always 2560×1440)

predictions(id, photo_id, class_id, confidence, bbox_xmin, bbox_ymin, bbox_xmax, bbox_ymax)
-- class_id: powdery_mildew | mirid | whitefly_aphid | miner_tuta | thrips | spider_mites
-- bbox: normalized [0,1], top-left (xmin,ymin) → bottom-right (xmax,ymax)
```

## API contract

**Spec-driven development.** `openapi.yaml` is the single source of truth.

- The spec is written first — never change code and then update the spec to match
- `openapi.yaml` is read-only during implementation; if a change is needed, update the spec first, then regenerate and update code
- Use `openapi-typescript` to generate frontend types from the spec — never write API types by hand
- Backend handlers must match the spec exactly (paths, methods, request/response shapes, status codes)

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/photos/{photoId}/upload-link` | Returns presigned PUT URL for MinIO |
| `GET` | `/photos` | Cursor-paginated; filters: `classId`, `minConfidence` |
| `GET` | `/photos/{photoId}` | Single photo with all predictions |

Auth: `X-API-Key` header on every request. Key set via `API_KEY` env var.

Filter semantics: a photo matches when a **single prediction** satisfies both `classId` AND `minConfidence` simultaneously. Always return all predictions for a matched photo.

**`originalUrl` presigning:** Generate a fresh presigned GET URL (1-hour TTL) for each photo on every API response — do not store the URL in the database. The frontend must not persist photo objects to localStorage or any long-lived cache; treat them as valid for the current session only.

## Docker

MinIO runs in Docker. Start the full local environment before running the backend or seed script:

```sh
docker compose --env-file .env.local up -d
```

- **MinIO S3 API:** `http://localhost:9000`
- **MinIO web console:** `http://localhost:9001` (minioadmin / minioadmin)
- The `minio-init` container creates the `scout` bucket automatically on first run.

The backend and seed script can assume MinIO is up and the bucket exists.

## Environment variables

See `.env.example`. Backend reads plain env vars; frontend reads `VITE_*` vars.

Key vars: `PORT`, `API_KEY`, `DB_PATH`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `MINIO_USE_SSL`, `VITE_API_URL`, `VITE_API_KEY`.

## What to build

### 1. Seed script
Upload every `dataset/images/*.jpg` to MinIO via `POST /photos/{photoId}/upload-link` → PUT. Re-runnable (idempotent by photo id).

- POST body must include `{ "contentType": "image/jpeg" }`
- The `UploadLink` response includes a `headers` map — forward every header from it onto the PUT request (this sets `Content-Type` on the MinIO object; omitting it stores objects as `application/octet-stream` and breaks browser rendering)

### 2. Thumbnail engine (most important)

**Decided URL interface** (not in `openapi.yaml` — thumbnail delivery is out of contract scope):

```
GET /thumbnails/{photoId}?w={width}&dpr={dpr}&fmt={fmt}
```

- `w`: CSS pixel width (e.g. 200, 400, 800) — required
- `dpr`: device pixel ratio, 1 | 2 | 3, default 1
- `fmt`: `webp` (preferred) | `jpeg` (fallback), default `webp`
- Server multiplies `w × dpr` to get the actual pixel dimension to generate

The frontend uses this to build `srcset` strings. bbox math uses the CSS pixel size (`w`), not `w × dpr`.

**Resource bounds** (server: ~1 vCPU, 512 MB–1 GB RAM; originals: 2560×1440 JPEG ~15 MB decoded each):

- Max **4 concurrent** thumbnail generations enforced via semaphore; return `503 Retry-After` when full
- **Singleflight** coalescing on `(photoId, w, dpr, fmt)` — identical in-flight requests share one generation
- **Disk cache**: LRU eviction, default 500 MB cap, configurable via `THUMB_CACHE_SIZE_MB` env var
- Cache key: `{photoId}_{w}_{dpr}_{fmt}` — simple, human-readable on disk

Expose cache hit/miss and generation time via `/metrics`.

### 3. Gallery
- Scrolling paginated grid; request thumbnails using `srcset`/`sizes`
- **Bounding boxes drawn at every size and DPR** — this is the hardest part
  - bbox is normalized [0,1]; multiply by the **rendered** element size, not the original
  - Boxes must stay accurate when the image scales
- Filter by `classId` and `minConfidence`
- Click to open full-size with predictions overlaid

### 4. Greenhouse map (bonus)
- Konva canvas, 40×40 m floor plan
- Each photo at its `x,y` position; zoom and pan
- Click a location to filter gallery to nearby photos
- Class filter shared between map and gallery (Redux state)

## Backend conventions

- Structured logs (JSON), one correlation/request id per request, no secrets in logs
- All errors: `{ request_id, message, code }` — shaped in one place, right HTTP status, no stack traces leaked
- `/metrics` endpoint: request rate, latency, error rate, thumbnail cache hit/miss, thumbnail generation time
- 4xx vs 5xx distinction is strict — validation failures are 400, not 500

## Frontend conventions

- Feature-based folders: `features/gallery/`, `features/map/`, `features/filters/`
- Redux Toolkit for shared state (filters, selected photo)
- Never show a blank screen or broken-image grid — always render loading, empty, and error states
- CSS Modules for all component styles
- Types generated from `openapi.yaml` via `openapi-typescript`

## Tests (required)

1. **bbox coordinate transform** — the core math: normalized → rendered px at any size/DPR
2. **Thumbnail request parse/validate + coordinate math**
3. **A key component or reducer** (e.g., filter reducer, gallery grid)
4. **One backend smoke test**: ingest a photo → read it back via `GET /photos/{photoId}`
5. Run tests before marking any work complete

All tests must pass from a clean clone with no manual setup beyond `make dev` or equivalent.

## Critical invariants

- **Never multiply bbox by the original image dimensions.** Multiply by the rendered element size.
- Thumbnail cache must be concurrency-safe (multiple simultaneous requests for the same key must not trigger multiple generations).
- `predictions.db` is read-only source of truth — do not write to it, do not copy it into another DB.
- The seed script must be idempotent — running it twice must not duplicate objects in MinIO.


## Git

- Branch names: `kebab-case` derived from feature name
- Never force-push main
- Commit messages: imperative mood, under 72 chars
- Never commit `.env` or secrets
- Always confirm with the user before destructive git commands
- Never add `Co-Authored-By` lines to commit messages