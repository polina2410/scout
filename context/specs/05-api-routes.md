# Spec 05 — API Routes

**Plan ref:** Phase 2, Step 5  
**Goal:** Implement the three API routes (`POST /photos/{photoId}/upload-link`, `GET /photos/{photoId}`, `GET /photos`) exactly as specified in `openapi.yaml`, behind `X-API-Key` auth, wired into `main.go` — so the full data API is operational.

---

## 1. `App` struct — `internal/handler/app.go`

Handlers are methods on a shared `App` struct that carries the two dependencies wired in `main.go`. This avoids threading individual dependencies through every handler signature.

```go
package handler

// App holds the shared dependencies for all API handlers.
type App struct {
    DB    *db.DB
    Store minio.Presigner
    Log   *slog.Logger
}
```

Imports: `internal/db`, `internal/minio`, `log/slog`.

---

## 2. Auth middleware — `internal/middleware/auth.go`

```go
// APIKeyAuth returns middleware that enforces X-API-Key on every request.
// Requests to /health bypass auth — that route is a liveness probe.
func APIKeyAuth(key string) func(http.Handler) http.Handler
```

**Behaviour:**
- If `r.URL.Path == "/health"`, call `next` immediately — no auth check
- Read `r.Header.Get("X-API-Key")`
- Compare to `key` using `subtle.ConstantTimeCompare` (prevents timing attacks)
- If missing or mismatched: `WriteError(w, r, 401, ErrCodeUnauthorized, "missing or invalid API key")` and return — do not call `next`

**Wire order in `main.go`** (outermost middleware runs first):

```go
var h http.Handler = mux
h = middleware.APIKeyAuth(cfg.APIKey)(h)  // inner — auth runs after correlation ID is set
h = middleware.CorrelationID(log)(h)      // outer — every request (including 401s) gets a request_id
```

> **Why CorrelationID is outermost:** `WriteError` reads the request ID from context via `RequestIDFromContext`. If auth ran first, rejected requests would have an empty `request_id` in the 401 body, violating the `ApiError` schema (`required: [request_id, message]`). CorrelationID must run first so every response — including auth failures — carries a correlation ID.

---

## 3. Response types — `internal/handler/response.go`

Go structs that serialise to the exact JSON shape in `openapi.yaml`. These are separate from the `db.*` types — the handler layer maps between them.

```go
type BoundingBoxResponse struct {
    XMin float64 `json:"xMin"`
    YMin float64 `json:"yMin"`
    XMax float64 `json:"xMax"`
    YMax float64 `json:"yMax"`
}

type PredictionResponse struct {
    ClassID    string              `json:"classId"`
    Confidence float64             `json:"confidence"`
    BBox       BoundingBoxResponse `json:"bbox"`
}

type PhotoResponse struct {
    ID          string               `json:"id"`
    X           float64              `json:"x"`
    Y           float64              `json:"y"`
    H           float64              `json:"h"`
    Width       int                  `json:"width"`
    Height      int                  `json:"height"`
    CapturedAt  string               `json:"capturedAt"`
    OriginalURL string               `json:"originalUrl"`
    Predictions []PredictionResponse `json:"predictions"`
}

type PhotoPageResponse struct {
    Items     []PhotoResponse `json:"items"`
    NextToken string          `json:"next_token,omitempty"`
}

type UploadLinkResponse struct {
    URL       string            `json:"url"`
    Method    string            `json:"method"`
    Headers   map[string]string `json:"headers,omitempty"`
    ExpiresAt string            `json:"expiresAt"`
}
```

**Mapping helper** (unexported, in `response.go`):

```go
func photoToResponse(p db.Photo) PhotoResponse
func predictionToResponse(p db.Prediction) PredictionResponse
```

---

## 4. `ValidationError` response — `internal/handler/handler.go`

The openapi.yaml `ValidationError` shape includes a `details` array that the base `WriteError` cannot produce. Add:

```go
type ValidationDetail struct {
    Field string `json:"field"`
    Issue string `json:"issue"`
}

type ValidationErrorResponse struct {
    RequestID string             `json:"request_id"`
    Message   string             `json:"message"`
    Code      string             `json:"code"`
    Details   []ValidationDetail `json:"details"`
}

// WriteValidationError writes a 400 response with per-field detail entries.
func WriteValidationError(w http.ResponseWriter, r *http.Request, details []ValidationDetail)
```

`WriteValidationError` sets `code: "ValidationError"` (matching the openapi.yaml enum), `message: "request validation failed"`, and populates `details`.

---

## 5. `POST /photos/{photoId}/upload-link` — `internal/handler/upload.go`

**Route:** `POST /photos/{photoId}/upload-link`

```go
func (a *App) CreateUploadLink(w http.ResponseWriter, r *http.Request)
```

**Steps:**

1. Extract `photoId` from `r.PathValue("photoId")` — validate it is a well-formed UUID using `uuid.Parse(photoId)` from `github.com/google/uuid` (already in the module graph via minio-go); return `400 ValidationError` with `field: "photoId", issue: "must be a valid UUID"` if invalid
2. Decode JSON body into `struct{ ContentType string \`json:"contentType"\` }`; return `400 ValidationError` with `field: "contentType", issue: "required"` on decode failure
3. Validate `contentType == "image/jpeg"`; return `400 ValidationError` with `field: "contentType", issue: "must be image/jpeg"` otherwise — this is the guard that keeps `ObjectKey`'s `.jpg` extension correct
4. Call `a.DB.GetPhoto(ctx, photoId)`; return `404 NotFound` with `resource_id: photoId` if `db.ErrNotFound`
5. Call `a.Store.PresignedPutURL(ctx, photoId, contentType, 15*time.Minute)`; return `500 InternalServerError` on error
6. Return `200` with `UploadLinkResponse{URL, Method: "PUT", Headers: headers, ExpiresAt: expiresAt.Format(time.RFC3339)}`

**Error table:**

| Condition | Status | Code |
|---|---|---|
| Invalid `photoId` format | 400 | `ValidationError` |
| Missing/malformed body | 400 | `ValidationError` |
| `contentType` not `image/jpeg` | 400 | `ValidationError` |
| Photo not in DB | 404 | `NotFound` |
| MinIO presign error | 500 | `InternalServerError` |

---

## 6. `GET /photos/{photoId}` — `internal/handler/photo.go`

**Route:** `GET /photos/{photoId}`

```go
func (a *App) GetPhoto(w http.ResponseWriter, r *http.Request)
```

**Steps:**

1. Extract `photoId` from `r.PathValue("photoId")`; validate with `uuid.Parse(photoId)` (same rule as §5); return `404 NotFound` on invalid format — an unrecognisable ID cannot exist in the DB
2. Call `a.DB.GetPhoto(ctx, photoId)`; return `404 NotFound` with `resource_id: photoId` if `db.ErrNotFound`
3. Call `a.Store.PresignedGetURL(ctx, photoId)`; return `500` on error
4. Map `db.Photo` → `PhotoResponse`, set `OriginalURL` from step 3
5. Return `200` with the mapped response

**Error table:**

| Condition | Status | Code |
|---|---|---|
| Invalid `photoId` format | 404 | `NotFound` |
| Photo not in DB | 404 | `NotFound` |
| MinIO presign error | 500 | `InternalServerError` |

---

## 7. `GET /photos` — `internal/handler/photos.go`

**Route:** `GET /photos`

```go
func (a *App) ListPhotos(w http.ResponseWriter, r *http.Request)
```

**Query parameter parsing and validation:**

| Param | Type | Validation | Default |
|---|---|---|---|
| `cursor` | string | opaque, pass through | `""` |
| `limit` | integer | 1 ≤ limit ≤ 200 (matches openapi.yaml `maximum: 200`) | 50 |
| `classId` | string | any value (DB layer filters) | `""` |
| `minConfidence` | float64 | 0.0 ≤ v ≤ 1.0 | 0 |

> **Limit and DB cap alignment:** `db.ListPhotos` now uses `maxPageLimit = 200` as its hard cap, matching this handler's upper bound. A handler-validated `limit=200` will pass through the DB layer unchanged.

Return `400 ValidationError` (with per-field details) for any out-of-range value. A missing param uses its default — do not error on absent params.

**Steps:**

1. Parse and validate query params as above
2. Call `a.DB.ListPhotos(ctx, db.ListParams{Cursor, Limit, ClassID, MinConfidence})`; return `500` on DB error
3. For each photo in the result, call `a.Store.PresignedGetURL(ctx, photo.ID)` concurrently — use `errgroup` or a simple goroutine fan-out with bounded concurrency; collect results into a `map[string]string` keyed by photo ID
4. Map each `db.Photo` → `PhotoResponse`, filling `OriginalURL` from the presign map
5. Return `200` with `PhotoPageResponse{Items, NextToken: nextCursor}`

**Concurrent presigning:** Up to 50 photos per page, each needing one presign call. Fan out with goroutines — presigning is a cheap HTTP signature operation but latency stacks badly in series. Cap concurrency at `min(len(photos), 10)` to avoid connection bursts.

**Error table:**

| Condition | Status | Code |
|---|---|---|
| Invalid query param value | 400 | `ValidationError` |
| DB error | 500 | `InternalServerError` |
| Any presign error | 500 | `InternalServerError` |

---

## 8. Route registration — `main.go`

Replace `_ = store` with full wiring:

```go
app := &handler.App{
    DB:    database,
    Store: store,
    Log:   log,
}

mux.HandleFunc("POST /photos/{photoId}/upload-link", app.CreateUploadLink)
mux.HandleFunc("GET /photos/{photoId}", app.GetPhoto)
mux.HandleFunc("GET /photos", app.ListPhotos)
```

Auth middleware wraps the full mux (the middleware itself skips `/health`):

```go
var h http.Handler = mux
h = middleware.CorrelationID(log)(h)
h = middleware.APIKeyAuth(cfg.APIKey)(h)
```

---

## 9. Tests — `internal/handler/`

Use `httptest.NewRecorder` + `httptest.NewServer` with a real in-memory SQLite (same helper as `db_test.go`) and a mock `Presigner`.

### Mock Presigner

```go
type mockPresigner struct {
    putURL  string
    getURL  string
    putErr  error
    getErr  error
}
func (m *mockPresigner) PresignedPutURL(...) (string, map[string]string, time.Time, error)
func (m *mockPresigner) PresignedGetURL(...) (string, error)
```

### Required test cases

| Test | Setup | Assert |
|---|---|---|
| `TestCreateUploadLink_OK` | photo exists in DB, mock presigner returns URL | 200, `url`/`method`/`expiresAt` in body |
| `TestCreateUploadLink_NotFound` | photo not in DB | 404, `code: "NotFound"`, `resource_id` present |
| `TestCreateUploadLink_BadContentType` | body `{"contentType":"image/png"}` | 400, `code: "ValidationError"`, details mention `contentType` |
| `TestCreateUploadLink_MissingBody` | no body / invalid JSON | 400, `code: "ValidationError"` |
| `TestGetPhoto_OK` | photo in DB, mock presigner | 200, `originalUrl` set, all predictions present |
| `TestGetPhoto_NotFound` | no such photo | 404, `code: "NotFound"` |
| `TestListPhotos_OK` | 3 photos in DB | 200, `items` length 3, `next_token` absent |
| `TestListPhotos_Filter` | class filter matches 1 of 3 | 200, `items` length 1 |
| `TestListPhotos_Pagination` | `limit=1` | 200, `next_token` present |
| `TestListPhotos_BadLimit` | `?limit=999` | 400, `code: "ValidationError"` |
| `TestAuth_Missing` | no `X-API-Key` header on any route | 401, `code: "AuthenticationRequired"` |
| `TestAuth_Wrong` | wrong key value | 401, `code: "AuthenticationRequired"` |
| `TestAuth_HealthBypass` | no key on `/health` | 200 (auth skipped) |

### Backend smoke test — `internal/handler/smoke_test.go`

Required by CLAUDE.md. Skipped when `MINIO_ENDPOINT` not set.

1. Start `httptest.NewServer` with real SQLite + real MinIO `Client`
2. `POST /photos/{id}/upload-link` → get presigned PUT URL and headers
3. PUT a minimal JPEG to the URL, forwarding headers
4. `GET /photos/{id}` → assert `id`, `originalUrl` non-empty, `predictions` correct

---

## Carry-forward items

From prior reviews (spec 03 and 04):

- **Auth middleware** — implemented in this step (§2)
- **`ObjectKey` photoID pre-validation** — enforced in §5 and §6 before any DB or MinIO call
- **`HealthResponse` struct** — replace `map[string]string` in the health handler with a named struct: `type HealthResponse struct { Status string \`json:"status"\`; Version string \`json:"version"\` }`
- **`ErrCodeTooManyRequests`** — add to `handler.go` constants now for use in the thumbnail step: `ErrCodeTooManyRequests = "TOO_MANY_REQUESTS"`

---

## Acceptance criteria

- [ ] `POST /photos/{photoId}/upload-link` returns 200 with `url`, `method: "PUT"`, `headers`, `expiresAt` for a known photo ID
- [ ] `POST /photos/{photoId}/upload-link` returns 400 when `contentType` is not `"image/jpeg"`
- [ ] `POST /photos/{photoId}/upload-link` returns 404 when `photoId` not in DB
- [ ] `GET /photos/{photoId}` returns 200 with `originalUrl` filled and all predictions
- [ ] `GET /photos/{photoId}` returns 404 for unknown ID
- [ ] `GET /photos` returns all photos with no filter; `next_token` absent when results fit in one page
- [ ] `GET /photos?classId=thrips` returns only photos with a thrips prediction
- [ ] `GET /photos?limit=1` returns one photo and a `next_token`
- [ ] `GET /photos?limit=999` returns 400 `ValidationError`
- [ ] Every route returns 401 when `X-API-Key` header is missing or wrong
- [ ] `GET /health` returns 200 without `X-API-Key` (auth bypass)
- [ ] All handler unit tests pass without a live MinIO
- [ ] Backend smoke test passes with live MinIO (skipped when `MINIO_ENDPOINT` not set)
- [ ] `go build ./...` and `go vet ./...` pass
- [ ] `go test ./internal/handler/...` passes
