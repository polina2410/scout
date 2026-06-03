# Features History

### project-structure-and-tooling
Scaffolded Go module, Makefile, Docker Compose for MinIO, `.env.example`, `go.sum`, and CI-ready project skeleton.

### config-logging-errors
Wired typed env-var config (`internal/config`), structured JSON logger (`internal/logger`), correlation-ID middleware (`internal/middleware`), and centralised error/JSON response helpers (`internal/handler`). Updated `main.go` to wire all components; `/health` returns `{"status":"ok","version":"dev"}` with `X-Request-ID` header.

### data-layer-sqlite
Implemented read-only SQLite data layer (`internal/db`): typed `Photo`, `Prediction`, `ClassID` consts, `ListParams`. `GetPhoto` returns `ErrNotFound` sentinel; `ListPhotos` uses keyset cursor pagination (`captured_at DESC, id DESC`) with a single-prediction subquery filter ensuring one prediction satisfies both `classId` AND `minConfidence`. Predictions for a full page loaded in one batch query. Wired `db.Open` into `main.go` with fail-fast on bad path. 12 tests covering happy path, not-found, all filter combos, malformed cursors, and empty results.
