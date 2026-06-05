# Scout

Greenhouse pest and disease monitoring. Cameras produce 2560×1440 JPEGs; a CV model annotates them with bounding boxes; Scout lets growers browse detections and locate problems on a floor map.

## What's built

- **Go backend** — REST API (`GET /photos`, `GET /photos/{id}`, `POST /photos/{id}/upload-link`), on-demand thumbnail engine with singleflight coalescing, disk LRU cache that survives restarts, per-IP rate limiting, structured logging, correlation IDs, `/metrics` endpoint
- **React + Vite frontend** — scrolling paginated gallery with per-card bbox overlays (canvas, DPR-aware), class/confidence filter bar, full-size photo modal, zoomable Konva greenhouse floor map with radius-based location filter
- **MinIO** — S3-compatible object storage, run locally via Docker Compose

## Prerequisites

| Tool | Version |
|---|---|
| Docker Desktop | any recent |
| Go | 1.22+ |
| Node.js | 20+ |
| pnpm | 9+ |

## Setup

### 1. Clone and configure

```sh
git clone <repo-url>
cd scout-takehome
cp .env.example .env.local
```

Edit `.env.local` — set `API_KEY` to any non-empty string (e.g. `dev-scout-key`). MinIO credentials can stay as the defaults.

### 2. Start MinIO

```sh
make dev
```

MinIO S3 API: `http://localhost:9000` · Console: `http://localhost:9001` (minioadmin / minioadmin)

### 3. Start the backend

From the repo root:

```sh
make server
```

This loads `.env.local` and starts the server on `http://localhost:8080`. The target detects the OS (PowerShell on Windows, inline env on macOS/Linux), so the same command works everywhere — no manual env-var loading needed.

> Prefer to run it by hand? Load `.env.local` into your shell, then `cd backend && go run ./cmd/server`. Run from the repo root so `DB_PATH=../dataset/predictions.db` resolves correctly.

### 4. Seed images

In a new terminal, from the repo root:

```sh
make seed
```

Uploads all 50 JPEGs from `dataset/images/` to MinIO. Re-running is safe — already-uploaded photos are skipped.

### 5. Start the frontend

In a new terminal:

```sh
cp .env.local frontend/.env.local
cd frontend
pnpm install
pnpm dev
```

Open the URL Vite reports (default `http://localhost:5173`).

## Tests

**Backend** — works without Docker (smoke test auto-skips when `MINIO_ENDPOINT` is unset):
```sh
cd backend && go test ./...
```

**Frontend:**
```sh
cd frontend && pnpm test
```

## Project layout

```
scout-takehome/
├── backend/           Go service — API, thumbnail engine, metrics, seed script
│   ├── cmd/server/    main entrypoint
│   ├── cmd/seed/      image ingestion tool
│   └── internal/      db, handler, minio, thumb, metrics, middleware
├── frontend/          React + Vite app
│   └── src/features/  gallery/, filters/, map/
├── dataset/
│   ├── images/        50 source JPEGs — seeded into MinIO, not served directly
│   └── predictions.db SQLite — read-only source of truth, never written to
├── openapi.yaml       API contract — source of truth for all route shapes
├── CLAUDE.md          AI build instructions
├── DECISIONS.md       Architecture decisions and spec gap resolutions
└── .env.example       All env vars with descriptions
```

## Environment variables

See `.env.example` for the full list. Key variables:

| Variable | Used by | Notes |
|---|---|---|
| `API_KEY` | backend + seed + frontend | Set the same value in `.env.local` and `frontend/.env.local` |
| `DB_PATH` | backend | Path to `predictions.db` — relative to `backend/` working dir |
| `MINIO_ENDPOINT` | backend + seed | `localhost:9000` for local Docker |
| `MINIO_ACCESS_KEY` | backend + seed | Default: `minioadmin` |
| `MINIO_SECRET_KEY` | backend + seed | Default: `minioadmin` |
| `MINIO_BUCKET` | backend + seed | Default: `scout` |
| `VITE_API_URL` | frontend | Backend base URL — must match where the backend is running |
| `VITE_API_KEY` | frontend | Must equal `API_KEY` |

Optional backend tuning (sensible defaults, override only if needed): `THUMB_CACHE_SIZE_MB` (disk cache cap, default 500), `THUMB_RATE_PER_SEC` / `THUMB_RATE_BURST` (per-IP rate limit on `/thumbnails`, default 30/60), `TRUST_PROXY_HEADERS` (set to `true` only when running behind a trusted reverse proxy — enables reading client IP from `X-Forwarded-For` / `X-Real-IP` for the rate limiter; default `false`).
