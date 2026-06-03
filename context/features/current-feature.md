# Current Feature: /metrics Endpoint

## Status
In Progress

## Goals

- New `internal/metrics` package with `Collector` (fixed-bucket histogram, status split, uptime), `Middleware`, and `Snapshot` type
- `Snapshot` exposes: `Total`, `Total2xx/4xx/5xx`, `UptimeSec`, `RequestsPerSec`, `ErrorRate`, `LatencyMeanMs`, `LatencyP50Ms`, `LatencyP95Ms`
- `internal/handler/metrics.go` — `ThumbSnapshot` struct + `MetricsHandler(col, func() ThumbSnapshot)` with no import cycle
- `GET /metrics` returns JSON with `requests` and `thumbnails` sections including `requests_per_sec`, `error_rate`, and `gen_p95_ms`
- Retroactive carry-forward in `internal/thumb/`: generation time histogram (`genBuckets`, `genBucketBoundsMs`), `genPercentileMs`, `GenP95Ms` added to `ThumbMetrics`
- Middleware wired between `CorrelationID` and `APIKeyAuth` — counts all requests including 401s
- All derived fields (`requests_per_sec`, `error_rate`, `cache_hit_rate`, `gen_mean_ms`, `gen_p95_ms`) return `0.0` (not NaN) in zero state
- 7 collector/middleware unit tests + 2 handler tests
- `go test ./internal/metrics/... ./internal/handler/... ./internal/thumb/...` passes

## Notes

- Spec: `context/specs/07-metrics-endpoint.md`
- `bucketBoundsMs` for request latency: `[1, 5, 10, 25, 50, 100, 250, 500, 1000]` ms
- `genBucketBoundsMs` for generation time: `[50, 100, 250, 500, 750, 1000, 2000, 3000, 5000]` ms — higher bounds match typical MinIO fetch + decode + resize + encode time
- Import cycle prevention: `handler` imports `internal/metrics` (new pkg); `thumb` is never imported by `handler` — thumbnail data passed via `func() ThumbSnapshot` closure in `main.go`
- Auth: no bypass — `GET /metrics` requires `X-API-Key` like all data routes
- Middleware order (outermost first): `CorrelationID → MetricsMiddleware → APIKeyAuth → mux`
- `[numGenBuckets]atomic.Int64` for generation histogram (lock-free per-bucket increments); `Metrics()` snapshot loads each bucket with one `Load()` call — no transactional consistency required
- `requests_per_sec` is `0.0` when `UptimeSec == 0` (startup edge case only)
- `TestCollector_RateAndErrorRate` must wait a non-zero duration after `NewCollector()` before taking the snapshot — use a fresh `Collector` with a fixed `startTime` set 1s in the past, or call `time.Sleep(1ms)` before asserting

## History

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

### thumbnail-endpoint
Implemented `GET /thumbnails/{photoId}?w={width}&dpr={dpr}&fmt={fmt}` in `internal/thumb/`. `ParseParams` validates all four params and collects all errors before returning. `DiskCache` is a size-bounded LRU using `container/list` + `sync.Mutex`; `safePath()` guards against key-based path traversal. `Service` uses `singleflight.Group` with a 4-slot semaphore inside the closure so N coalesced requests consume exactly 1 generation slot. `generate` fetches via `Downloader` interface, decodes JPEG, scales with `golang.org/x/image/draw.CatmullRom`, and encodes as JPEG (WebP requires CGO — `effectiveFormat()` normalises the cache key to `"jpeg"` until a C compiler is available). `ErrObjectNotFound` on `*minio.Client` uses `errors.New`; `GetOriginal` triggers the MinIO GET via `Stat()`. Auth middleware updated to bypass `/thumbnails/`. API key comparison upgraded to SHA-256 hash comparison to prevent length timing leak. `Content-Length` set on all image responses. 22 tests (9 param, 5 cache, 8 handler; mock Downloader, no live MinIO required).
