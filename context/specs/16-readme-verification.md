# Spec 16 — README & Verification

**Plan ref:** Phase 9–10, Steps 16–17  
**Goal:** Write a `README.md` that lets a fresh clone reach a running app with one terminal session, and confirm all tests pass cleanly.

---

## 1. Required tests audit — already complete

All four required tests from the plan are present:

| Requirement | File | Tests |
|---|---|---|
| bbox coordinate transform | `frontend/src/features/gallery/bboxUtils.test.ts` | 5 tests — normalized → CSS px at any size |
| Thumbnail parse/validate + w×dpr math | `backend/internal/thumb/params_test.go` | 9 tests — `ParseParams`, `PxWidth = w×dpr`, clamping |
| Filter reducer | `frontend/src/features/filters/filtersSlice.test.ts` | 11 reducer tests |
| Backend smoke: seed → GET | `backend/internal/handler/smoke_test.go` | `TestSmoke_IngestAndRead` — skipped without `MINIO_ENDPOINT` |

No new tests are needed for this step.

---

## 2. `README.md` — repo root

Write `README.md` at the repo root covering:

### Prerequisites section

List exact required tools with minimum versions:
- Docker Desktop (for MinIO)
- Go 1.22+
- Node.js 20+ and pnpm 9+

### Setup section (step-by-step)

```markdown
## Setup

1. Clone the repo and copy the env file:
   git clone <repo>
   cd scout-takehome
   cp .env.example .env.local
   # Edit .env.local — set API_KEY to any non-empty string

2. Start MinIO:
   make dev

3. Start the backend (new terminal):
   # Load env vars first:
   # macOS/Linux:  set -a && source .env.local && set +a
   # Windows PS:   foreach ($l in Get-Content .env.local) { if ($l -match '^([^#][^=]*)=(.*)$') { [System.Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim()) } }
   cd backend && go run ./cmd/server

4. Seed images (new terminal, same env vars loaded):
   cd backend && go run ./cmd/seed

5. Start the frontend (new terminal):
   cp .env.local frontend/.env.local
   cd frontend && pnpm install && pnpm dev

6. Open http://localhost:5173 (or the port Vite reports)
```

### Running tests section

```markdown
## Tests

# Backend (from repo root):
cd backend && go test ./...

# Frontend (from repo root):
cd frontend && pnpm test
```

### Architecture section

Brief description (3–5 bullet points):
- Go backend: REST API (`GET /photos`, `GET /photos/{id}`, `POST /photos/{id}/upload-link`), thumbnail engine with disk LRU cache, MinIO object storage
- React + Vite frontend: gallery grid with bbox overlays, class/confidence filters, photo modal, greenhouse floor map (react-konva)
- SQLite `predictions.db`: read-only source of truth — do not write to it
- All photo originals stored in MinIO; thumbnails generated on-demand and cached on disk

---

## 3. Verify `.env.example` completeness

Check every env var read by the codebase is documented in `.env.example`. Current file covers: `PORT`, `API_KEY`, `DB_PATH`, `MINIO_*`, `THUMB_CACHE_SIZE_MB`, `LOG_LEVEL`, `VITE_API_URL`, `VITE_API_KEY`, `API_URL`, `IMAGES_DIR`. Confirm nothing is missing.

---

## Acceptance criteria

- [ ] `README.md` exists at repo root with prerequisites, setup steps, and test commands
- [ ] `go test ./...` from `backend/` passes cleanly (smoke test skipped without env)
- [ ] `pnpm test` from `frontend/` passes — all 52 tests green
- [ ] `pnpm build` from `frontend/` passes — no TypeScript errors
- [ ] `pnpm lint` from `frontend/` passes — no errors
- [ ] `.env.example` covers every env var consumed by the app
- [ ] README setup steps are accurate and complete — a developer with the prerequisites can reach the running app by following them exactly
