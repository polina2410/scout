# Spec 02 — Config, Logging & Error Handling

**Plan ref:** Phase 2, Step 2
**Goal:** Wire a production-grade foundation into the Go server — typed config loaded at startup, structured JSON logging via `log/slog`, correlation ID middleware on every request, and a centralized error response helper — so all subsequent steps can build on consistent observability and error contracts.

---

## 1. Environment variables

Complete list of env vars the server reads. Add `LOG_LEVEL` to `.env.example`.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `PORT` | No | `8080` | HTTP listen port |
| `API_KEY` | Yes | — | `X-API-Key` header value; never logged |
| `DB_PATH` | Yes | — | Path to `predictions.db` (read-only) |
| `MINIO_ENDPOINT` | Yes | — | e.g. `localhost:9000` |
| `MINIO_ACCESS_KEY` | Yes | — | Never logged |
| `MINIO_SECRET_KEY` | Yes | — | Never logged |
| `MINIO_BUCKET` | Yes | — | e.g. `scout` |
| `MINIO_USE_SSL` | No | `false` | Parse as bool; `"true"` enables TLS |
| `THUMB_CACHE_SIZE_MB` | No | `500` | Parse as int64 |
| `LOG_LEVEL` | No | `info` | Accepted values: `debug`, `info`, `warn`, `error` |

---

## 2. `internal/config` package

### Struct

```go
package config

// Config holds all runtime configuration for the server.
// All fields are populated by Load(); callers must treat it as read-only.
type Config struct {
    Port             string
    APIKey           string
    DBPath           string
    MinIOEndpoint    string
    MinIOAccessKey   string
    MinIOSecretKey   string
    MinIOBucket      string
    MinIOUseSSL      bool
    ThumbCacheSizeMB int64
    LogLevel         string
}
```

### Load function

```go
// Load reads environment variables and returns a populated Config.
// It returns an error listing every missing required variable — not just the first —
// so operators see all gaps in a single restart cycle.
func Load() (*Config, error)
```

**Behaviour:**

- Collect all missing required vars into a slice; return a single `fmt.Errorf` that lists them all, e.g.:
  `required env vars not set: API_KEY, DB_PATH, MINIO_ENDPOINT`
- `MINIO_USE_SSL`: accept `"true"` (case-insensitive) as `true`, anything else as `false`. Never error on an unrecognised value.
- `THUMB_CACHE_SIZE_MB`: parse with `strconv.ParseInt`; on parse failure return an error: `THUMB_CACHE_SIZE_MB must be a positive integer, got "<value>"`.
- `PORT`: default `"8080"` if blank. No validation beyond non-empty.
- `LOG_LEVEL`: default `"info"` if blank. Normalise to lowercase. Validation happens in the logger setup, not here.
- `APIKey`, `MinIOAccessKey`, `MinIOSecretKey` must never appear in log output at any level. Config loading code must not log these fields.

---

## 3. Logger — `log/slog`

### Setup (called once in `main`)

Lives in `internal/logger` (see `DECISIONS.md` #6).

```go
// New constructs a slog.Logger with a JSON handler writing to w.
// level must be one of "debug", "info", "warn", "error" (case-insensitive).
// An unrecognised level defaults to info and does not error.
func New(w io.Writer, level string) *slog.Logger
```

**Rationale for a passed-in logger (not a global):** A single `*slog.Logger` created in `main` and threaded through middleware and handlers makes the dependency explicit, simplifies testing (pass `io.Discard`), and avoids `init()`-order surprises. Handlers receive it via a small `App` struct (see §6). This is preferred over `slog.SetDefault` because it keeps the wiring visible.

**Log levels:**

| `level` string | `slog.Level` |
|---|---|
| `"debug"` | `slog.LevelDebug` |
| `"info"` | `slog.LevelInfo` |
| `"warn"` | `slog.LevelWarn` |
| `"error"` | `slog.LevelError` |
| anything else | `slog.LevelInfo` |

**Standard log fields** (present on every structured log line):

| Field | Type | Description |
|---|---|---|
| `time` | RFC3339 string | Provided by slog's JSON handler automatically |
| `level` | string | `"INFO"`, `"ERROR"`, etc. |
| `msg` | string | Human-readable message |
| `request_id` | string | From context; `""` if not in a request context |

**Access log line** (emitted by the correlation middleware after the handler returns):

```json
{
  "time": "2026-06-03T10:00:00Z",
  "level": "INFO",
  "msg": "request",
  "request_id": "a3f2c1d4e5b6",
  "method": "GET",
  "path": "/health",
  "status": 200,
  "duration_ms": 1
}
```

Field names: `method` (string), `path` (string), `status` (int), `duration_ms` (int64).

---

## 4. Correlation ID middleware — `internal/middleware`

### Context key

```go
package middleware

// contextKey is an unexported type for context keys in this package.
// Using a named type prevents collisions with keys from other packages.
type contextKey int

const requestIDKey contextKey = 0
```

### Functions

```go
// RequestIDFromContext retrieves the request ID stored by CorrelationID.
// Returns "" if not set.
func RequestIDFromContext(ctx context.Context) string

// CorrelationID is an HTTP middleware that:
//  1. Checks for an incoming X-Request-ID header; uses it if present and non-empty.
//  2. Otherwise generates a new 12-byte random hex string (24 hex chars).
//  3. Stores the ID in the request context under requestIDKey.
//  4. Sets X-Request-ID on the response before the handler runs.
//  5. Wraps the ResponseWriter to capture the status code for access logging.
//  6. After the handler returns, logs one access line via logger (see §3).
//
// logger is the *slog.Logger constructed in main.
func CorrelationID(logger *slog.Logger) func(http.Handler) http.Handler
```

**ID generation:** Use `crypto/rand.Read` into a 12-byte buffer, then `hex.EncodeToString`. Result is a 24-character lowercase hex string. Do not use `math/rand`. Do not format as a hyphenated UUID — plain hex is sufficient and avoids the `github.com/google/uuid` dependency.

**Response writer wrapper** (unexported, defined in the same file):

```go
type responseWriter struct {
    http.ResponseWriter
    status int
    wrote  bool
}

func (rw *responseWriter) WriteHeader(code int) {
    if !rw.wrote {
        rw.status = code
        rw.wrote = true
        rw.ResponseWriter.WriteHeader(code)
    }
}
```

If `WriteHeader` is never called (handler writes body directly), default `status` to `200` before logging.

**Middleware must not modify** the request body or any header other than setting `X-Request-ID` on the response.

---

## 5. Error response — `internal/handler`

### Response shape

All non-2xx responses from the API use this JSON body:

```json
{
  "request_id": "a3f2c1d4e5b6",
  "message": "photo not found",
  "code": "NOT_FOUND"
}
```

Fields:

| Field | Type | Notes |
|---|---|---|
| `request_id` | string | Retrieved via `middleware.RequestIDFromContext(r.Context())` |
| `message` | string | Human-readable; safe to display in a UI |
| `code` | string | Machine-readable constant (see below) |

### Go struct and helpers

```go
package handler

// ErrorResponse is the JSON body returned for all non-2xx responses.
type ErrorResponse struct {
    RequestID string `json:"request_id"`
    Message   string `json:"message"`
    Code      string `json:"code"`
}

// WriteError writes a JSON error response.
// status is the HTTP status code (e.g. http.StatusNotFound).
// code is one of the named Err* constants in this package.
// message is a safe, human-readable description.
// It sets Content-Type: application/json and never leaks stack traces.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code string, message string)

// WriteJSON writes a 2xx JSON response.
// status is the HTTP status code. v is JSON-encoded into the body.
func WriteJSON(w http.ResponseWriter, status int, v any)
```

`WriteError` implementation notes:
- Call `w.Header().Set("Content-Type", "application/json")` before `w.WriteHeader(status)`.
- Populate `RequestID` from `middleware.RequestIDFromContext(r.Context())`.
- Encode with `json.NewEncoder(w).Encode(...)`. Do not pretty-print.
- If JSON encoding fails (extremely unlikely), fall back to a plain-text `500` — do not panic.

### Named error codes (string constants)

```go
const (
    ErrCodeBadRequest          = "BAD_REQUEST"           // 400 — malformed input, missing param, failed validation
    ErrCodeUnauthorized        = "UNAUTHORIZED"          // 401 — missing or invalid X-API-Key
    ErrCodeNotFound            = "NOT_FOUND"             // 404 — resource does not exist
    ErrCodeConflict            = "CONFLICT"              // 409 — e.g. photo already exists
    ErrCodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"  // 422 — structurally valid but semantically wrong
    ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"   // 503 — semaphore full (thumbnail engine)
    ErrCodeInternal            = "INTERNAL_ERROR"        // 500 — unexpected server error
)
```

**4xx vs 5xx rule (non-negotiable):** Any error arising from invalid or missing input data is 4xx. Only errors the server cannot anticipate or recover from are 5xx. Specifically:
- Missing/invalid query params → `400 BAD_REQUEST`
- Missing/wrong `X-API-Key` → `401 UNAUTHORIZED`
- Unknown `photoId` → `404 NOT_FOUND`
- DB or MinIO failures → `500 INTERNAL_ERROR`
- `message` for `500` responses must not include raw Go error strings; use `"an internal error occurred"`.

---

## 6. Updated `main.go`

Full replacement for `backend/cmd/server/main.go`:

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    "github.com/polina2410/scout/backend/internal/config"
    "github.com/polina2410/scout/backend/internal/handler"
    "github.com/polina2410/scout/backend/internal/logger"
    "github.com/polina2410/scout/backend/internal/middleware"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        slog.Error("config error", "error", err)
        os.Exit(1)
    }

    log := logger.New(os.Stdout, cfg.LogLevel)

    mux := http.NewServeMux()

    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        handler.WriteJSON(w, http.StatusOK, map[string]string{
            "status":  "ok",
            "version": "dev",
        })
    })

    var h http.Handler = mux
    h = middleware.CorrelationID(log)(h)

    log.Info("server starting", "port", cfg.Port)
    if err := http.ListenAndServe(":"+cfg.Port, h); err != nil {
        log.Error("server stopped", "error", err)
        os.Exit(1)
    }
}
```

Notes:
- `logger.New` lives in `internal/logger` — see `DECISIONS.md` #6.
- Before `slog` is initialised, the only logging is via `slog.Error` with the default handler (goes to stderr). This is the single acceptable use of the default logger.

---

## 7. `/health` endpoint update

The health handler must return:

```json
{"status":"ok","version":"dev"}
```

- HTTP `200 OK`
- `Content-Type: application/json`
- `X-Request-ID` response header set by middleware
- One access log line emitted by the middleware (not the handler itself)

`"version"` is the string literal `"dev"` for now. A future step may wire in a build-time variable via `-ldflags`.

---

## Acceptance criteria

- [ ] `config.Load()` returns a descriptive error listing **all** missing required vars when more than one is absent — not just the first
- [ ] `config.Load()` returns a descriptive error when `THUMB_CACHE_SIZE_MB` is set to a non-integer value
- [ ] `config.Load()` succeeds and sets `MinIOUseSSL = true` when `MINIO_USE_SSL=TRUE` (case-insensitive)
- [ ] `config.Load()` defaults `Port` to `"8080"` and `ThumbCacheSizeMB` to `500` when those vars are absent
- [ ] Server exits non-zero immediately on startup if any required var is missing
- [ ] All log output is valid JSON (each line parseable independently)
- [ ] Every request log line contains `request_id`, `method`, `path`, `status`, `duration_ms`
- [ ] `curl -s localhost:8080/health` returns `{"status":"ok","version":"dev"}` and the response includes an `X-Request-ID` header
- [ ] Sending `X-Request-ID: custom-id-123` on a request causes the same value to appear in the response `X-Request-ID` header and in the access log
- [ ] Two simultaneous requests to `/health` produce two log lines with different `request_id` values
- [ ] `WriteError` with status `500` and code `ErrCodeInternal` produces `{"request_id":"...","message":"an internal error occurred","code":"INTERNAL_ERROR"}` — no Go error string in the message
- [ ] `APIKey`, `MinIOAccessKey`, and `MinIOSecretKey` values do not appear anywhere in log output at any level
- [ ] `go vet ./...` and `go build ./...` pass with no errors
- [ ] `make test` exits 0 (no new tests required at this step; existing zero-test suite must still pass)
