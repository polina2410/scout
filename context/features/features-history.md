# Features History

### project-structure-and-tooling
Scaffolded Go module, Makefile, Docker Compose for MinIO, `.env.example`, `go.sum`, and CI-ready project skeleton.

### config-logging-errors
Wired typed env-var config (`internal/config`), structured JSON logger (`internal/logger`), correlation-ID middleware (`internal/middleware`), and centralised error/JSON response helpers (`internal/handler`). Updated `main.go` to wire all components; `/health` returns `{"status":"ok","version":"dev"}` with `X-Request-ID` header.

### data-layer-sqlite
Implemented read-only SQLite data layer (`internal/db`): typed `Photo`, `Prediction`, `ClassID` consts, `ListParams`. `GetPhoto` returns `ErrNotFound` sentinel; `ListPhotos` uses keyset cursor pagination (`captured_at DESC, id DESC`) with a single-prediction subquery filter ensuring one prediction satisfies both `classId` AND `minConfidence`. Predictions for a full page loaded in one batch query. Wired `db.Open` into `main.go` with fail-fast on bad path. 12 tests covering happy path, not-found, all filter combos, malformed cursors, and empty results.

### minio-integration
Implemented MinIO client package (`internal/minio`): `ObjectKey`, `Presigner` interface, `Client`/`New` with 5-second bucket-check timeout, `PresignedPutURL` (caller TTL capped at 1 hour, returns headers map caller must forward), `PresignedGetURL` (fixed 1-hour TTL, not on interface). Wired into `main.go` with fail-fast. Applied carry-forwards: `WriteJSON[T any]` generic, `const version`, `http.Server` timeouts. Updated plan-writing skill to write specs. Tests: `TestObjectKey`, `TestNew_Unreachable`, `TestPresignedRoundTrip` (PUT→GET Content-Type round-trip), `TestNew_BucketNotFound`, `TestPresignedPutURL_TTLTooLarge`.

### api-routes
Implemented the full data API surface. `App` struct (`internal/handler/app.go`) holds `*db.DB`, `minio.Presigner`, and `*slog.Logger`. `APIKeyAuth` middleware (`internal/middleware/auth.go`) uses `subtle.ConstantTimeCompare`, skips `/health`, and panics on empty key. Response types in `response.go` map `db.*` structs to openapi.yaml JSON shapes. `WriteValidationError` and `WriteNotFoundError` produce the spec's `details` array and `resource_id` fields respectively. `POST /photos/{photoId}/upload-link` validates UUID (400) + content-type (400), DB-checks existence (404), and presigns a 15-min PUT URL. `GET /photos/{photoId}` returns 404 for invalid UUID or missing photo. `GET /photos` validates query params, pages via DB, and fans out presigning across up to 10 goroutines. CorrelationID is outermost middleware; APIKeyAuth is inner so every 401 carries a `request_id`. 13 handler unit tests (mock Presigner + in-memory SQLite) + smoke test that skips without `MINIO_ENDPOINT`. Exported `db.NewDB` for cross-package test use.
