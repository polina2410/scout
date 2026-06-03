# Current Feature: Config, Logging & Error Handling

## Status
In Progress

## Goals

- `config.Load()` collects all missing required vars and returns a single error listing them all
- `config.Load()` returns an error when `THUMB_CACHE_SIZE_MB` is a non-integer
- `config.Load()` parses `MINIO_USE_SSL` case-insensitively; defaults `Port` to `"8080"` and `ThumbCacheSizeMB` to `500`
- Server exits non-zero on startup if any required env var is missing
- All log output is valid JSON (each line parseable independently)
- Every request log line contains `request_id`, `method`, `path`, `status`, `duration_ms`
- `GET /health` returns `{"status":"ok","version":"dev"}` with `Content-Type: application/json` and `X-Request-ID` header
- Incoming `X-Request-ID` header is echoed back in the response and appears in the access log
- Two simultaneous requests produce two log lines with different `request_id` values
- `WriteError` with `ErrCodeInternal` produces `{"request_id":"...","message":"an internal error occurred","code":"INTERNAL_ERROR"}` — no raw Go error string
- `APIKey`, `MinIOAccessKey`, `MinIOSecretKey` never appear in log output at any level
- `go vet ./...` and `go build ./...` pass; `make test` exits 0

## Notes

- New packages to create: `internal/config`, `internal/logger`, `internal/middleware` (correlation ID), `internal/handler` (error/JSON helpers)
- `internal/logger` exposes `New(w io.Writer, level string) *slog.Logger` — no global logger, wired through `main`
- Correlation ID: 12-byte `crypto/rand` → 24-char lowercase hex; accepts incoming `X-Request-ID` if present
- `responseWriter` wrapper captures status code for access logging; defaults to 200 if `WriteHeader` never called
- `WriteError` must set `Content-Type` before `WriteHeader`; fall back to plain-text 500 if JSON encoding fails
- Named error code constants live in `internal/handler`; 4xx for input errors, 5xx only for unexpected server errors
- `main.go` is a full replacement per spec §6 — uses `slog.Error` (default handler) only before the logger is initialized
- `"version": "dev"` in health response is a string literal for now

## History

### project-structure-and-tooling
Scaffolded Go module, Makefile, Docker Compose for MinIO, `.env.example`, `go.sum`, and CI-ready project skeleton.
