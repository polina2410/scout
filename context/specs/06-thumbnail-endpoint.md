# Spec 06 — Thumbnail Endpoint

**Plan ref:** Phase 3, Step 6  
**Goal:** Implement `GET /thumbnails/{photoId}?w={width}&dpr={dpr}&fmt={fmt}` — an on-demand thumbnail service that fetches originals from MinIO, resizes in Go, caches results to disk with LRU eviction, coalesces identical in-flight requests via singleflight, and enforces a 4-slot semaphore to bound CPU/memory use during image generation.

---

## 1. Package layout — `internal/thumb/`

All new code lives in `internal/thumb/`. Do not add anything to existing packages except the two additions in §2.

| File | Content |
|------|---------|
| `internal/thumb/params.go` | `Params` struct + `ParseParams` |
| `internal/thumb/cache.go` | `DiskCache` + `NewDiskCache` |
| `internal/thumb/service.go` | `Service`, `New`, `Handle`, `Metrics`, `Downloader` interface |
| `internal/thumb/params_test.go` | param parsing unit tests |
| `internal/thumb/cache_test.go` | cache unit tests |
| `internal/thumb/service_test.go` | handler unit tests with mock Downloader |

---

## 2. Additions to existing packages

### `internal/minio/minio.go`

Add one sentinel and one method to `*Client` — no interface changes to `Presigner`.

```go
// ErrObjectNotFound is returned by GetOriginal when the object does not exist in the bucket.
var ErrObjectNotFound = errors.New("object not found in bucket")

// GetOriginal streams the original JPEG bytes for the given photo.
// Returns ErrObjectNotFound (wrapped) if the object does not exist.
// Caller must close the returned ReadCloser.
func (c *Client) GetOriginal(ctx context.Context, photoID string) (io.ReadCloser, error)
```

Implementation: call `c.mc.GetObject(ctx, c.bucket, ObjectKey(photoID), miniogo.GetObjectOptions{})`. Inspect the MinIO error with `miniogo.ToErrorResponse(err).Code == "NoSuchKey"` — if so, return `fmt.Errorf("%w", ErrObjectNotFound)`.

### `internal/middleware/auth.go`

Update the bypass check to include thumbnails — browsers cannot attach `X-API-Key` to `<img src>` requests:

```go
// Before:
if r.URL.Path == "/health" {

// After:
if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/thumbnails/") {
```

---

## 3. Request params — `internal/thumb/params.go`

```go
// Params holds the validated parameters for a single thumbnail request.
type Params struct {
    PhotoID string // bare UUID (already validated)
    W       int    // CSS pixel width (1–2560)
    DPR     int    // device pixel ratio (1, 2, or 3)
    Fmt     string // "webp" or "jpeg"
    PxWidth int    // W × DPR, clamped to 2560
}

// ParseParams parses and validates query parameters.
// Returns a non-empty details slice when any param is invalid — caller must
// call handler.WriteValidationError and return without using Params.
func ParseParams(photoID string, q url.Values) (Params, []handler.ValidationDetail)
```

Validation rules:

| Param | Rule | Default | Error field |
|-------|------|---------|-------------|
| `photoId` | `uuid.Parse` must succeed | — | `"photoId"` |
| `w` | required; integer; 1 ≤ w ≤ 2560 | — | `"w"` |
| `dpr` | 1, 2, or 3 | `1` | `"dpr"` |
| `fmt` | `"webp"` or `"jpeg"` | `"webp"` | `"fmt"` |

Set `PxWidth = W × DPR`. If `PxWidth > 2560`, clamp to `2560` — originals are always 2560 px wide and upscaling is wasteful.

Collect all errors into the slice (do not stop at first) so the client sees all problems in one response.

---

## 4. Disk cache — `internal/thumb/cache.go`

```go
// DiskCache is a thread-safe, size-bounded LRU disk cache.
// Entries are files on disk; an in-memory index tracks sizes for eviction.
type DiskCache struct {
    dir     string
    maxSize int64       // bytes
    mu      sync.Mutex
    index   map[string]*cacheEntry
    lru     list.List   // container/list; front = most recently used
    total   int64       // current total bytes on disk
}

type cacheEntry struct {
    key  string
    size int64
    el   *list.Element
}

// NewDiskCache creates (or reuses) a cache directory.
// Returns an error if dir cannot be created.
func NewDiskCache(dir string, maxSize int64) (*DiskCache, error)
```

**Cache key format:** `{photoId}_{w}_{dpr}_{fmt}` — e.g. `11111111-1111-1111-1111-111111111111_400_2_webp`. This is both the in-memory index key and the filename on disk (no subdirectory).

**`Get(key string) ([]byte, bool)`**
1. Lock; check index — if absent, unlock and return `nil, false`
2. Unlock; `os.ReadFile(filepath.Join(dir, key))` — on read error (corrupt/deleted file), lock, remove from index, unlock, return `nil, false`
3. Lock; move the entry to the front of the LRU list; unlock
4. Return `data, true`

**`Put(key string, data []byte) error`**
1. `os.WriteFile(filepath.Join(dir, key), data, 0o644)` — return error if write fails
2. Lock; if key already in index, remove its old size from `total` and unlink from list
3. Evict LRU entries (back of list) until `total + len(data) <= maxSize`: delete the file and remove from index, update `total`
4. Add new entry to front of list, update `total`; unlock

**`Stats() (entries int, totalBytes int64)`** — locked snapshot for `/metrics` (Step 7).

No TTL: entries are evicted only by size pressure. The `Cache-Control` header on HTTP responses controls browser-side expiry.

---

## 5. `Downloader` interface and `Service` — `internal/thumb/service.go`

```go
// Downloader fetches original photo bytes from object storage.
// Defined here so the thumb package owns the interface it needs;
// *minio.Client satisfies it without importing this package.
type Downloader interface {
    GetOriginal(ctx context.Context, photoID string) (io.ReadCloser, error)
}

const (
    maxConcurrent = 4   // semaphore capacity — max simultaneous image generations
    retryAfterSec = 5   // Retry-After seconds when semaphore is full
    jpegQuality   = 85  // JPEG encode quality [0,100]
    webpQuality   = 80  // WebP encode quality [0,100]
)

// Service handles thumbnail request routing, caching, and concurrency control.
type Service struct {
    store Downloader
    cache *DiskCache
    sem   chan struct{}        // buffered channel of capacity maxConcurrent
    group singleflight.Group
    log   *slog.Logger

    // counters read by Metrics() — all updated with atomic methods
    hits   atomic.Int64 // cache hits
    misses atomic.Int64 // cache misses
    genOK  atomic.Int64 // successful generations
    genErr atomic.Int64 // failed generations
    genNs  atomic.Int64 // cumulative generation nanoseconds
}

// New creates a Service.
func New(store Downloader, cache *DiskCache, log *slog.Logger) *Service {
    return &Service{
        store: store,
        cache: cache,
        sem:   make(chan struct{}, maxConcurrent),
        log:   log,
    }
}

// ThumbMetrics is a snapshot of thumbnail service counters for the /metrics endpoint.
type ThumbMetrics struct {
    CacheHits    int64
    CacheMisses  int64
    GenOK        int64
    GenErr       int64
    GenTotalNs   int64 // divide by GenOK for mean generation time
}

// Metrics returns a point-in-time snapshot of the service counters.
func (s *Service) Metrics() ThumbMetrics
```

---

## 6. Handler flow — `(*Service).Handle`

Route registered in `main.go` as `GET /thumbnails/{photoId}`.

```go
func (s *Service) Handle(w http.ResponseWriter, r *http.Request)
```

**Steps:**

1. `p, details := ParseParams(r.PathValue("photoId"), r.URL.Query())`  
   If `len(details) > 0`: `handler.WriteValidationError(w, r, details)` and return

2. Build cache key:  
   `key := fmt.Sprintf("%s_%d_%d_%s", p.PhotoID, p.W, p.DPR, p.Fmt)`

3. If `data, ok := s.cache.Get(key); ok`:  
   `s.hits.Add(1)` → serve `data` with headers (§7, `X-Cache: HIT`) and return

4. `s.misses.Add(1)`

5. Run singleflight — all callers waiting for the same key share one generation:
   ```go
   val, err, _ := s.group.Do(key, func() (any, error) {
       select {
       case s.sem <- struct{}{}:
           defer func() { <-s.sem }()
       default:
           return nil, errAtCapacity  // unexported sentinel
       }
       return s.generate(r.Context(), p)
   })
   ```

6. Error routing:
   - `err == errAtCapacity` → `w.Header().Set("Retry-After", "5")`, `WriteError` 503 `ErrCodeServiceUnavailable`
   - `errors.Is(err, minioclient.ErrObjectNotFound)` → `WriteNotFoundError(w, r, p.PhotoID)`
   - any other err → `s.log.Error(...)`, `WriteError` 500 `ErrCodeInternal`

7. On success: `data := val.([]byte)`  
   `s.cache.Put(key, data)` — log but do not fail the request on cache-write error

8. Serve `data` with headers (§7, `X-Cache: MISS`)

**`generate(ctx context.Context, p Params) ([]byte, error)`:**

1. `t0 := time.Now()`
2. `rc, err := s.store.GetOriginal(ctx, p.PhotoID)` — return err on failure
3. `defer rc.Close()`
4. `src, err := jpeg.Decode(rc)` — return `fmt.Errorf("decode JPEG: %w", err)` on failure
5. Compute target bounds:  
   `dstW := p.PxWidth`  
   `dstH := src.Bounds().Dy() * dstW / src.Bounds().Dx()`  
   `dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))`  
   `draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)`
6. Encode:
   - `p.Fmt == "webp"`: `webp.Encode(&buf, dst, &webp.Options{Quality: webpQuality})`
   - `p.Fmt == "jpeg"`: `jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality})`
   - Return `fmt.Errorf("encode %s: %w", p.Fmt, err)` on failure
7. `s.genNs.Add(time.Since(t0).Nanoseconds())`; `s.genOK.Add(1)`
8. Return `buf.Bytes(), nil`

On any error in steps 2–6: `s.genErr.Add(1)` before returning.

---

## 7. HTTP response headers

Set on **every** response that serves image bytes (both cache hit and miss):

```
Content-Type:  image/webp        (or image/jpeg when fmt=jpeg)
Cache-Control: public, max-age=3600
X-Cache:       HIT               (or MISS on first generation)
```

---

## 8. New dependencies

```sh
cd backend
go get golang.org/x/image
go get github.com/chai2010/webp
```

| Package | Purpose | Notes |
|---------|---------|-------|
| `golang.org/x/image/draw` | `CatmullRom` resampling kernel | Part of `golang.org/x/image`; pure Go |
| `github.com/chai2010/webp` | WebP encode | CGO; bundles libwebp C sources — no system library required |

`golang.org/x/sync/singleflight` is already in the module graph (added during Step 5 `go mod tidy`).

---

## 9. Wiring — `main.go`

```go
import (
    "path/filepath"
    "os"

    "github.com/polina2410/scout/backend/internal/thumb"
    minioclient "github.com/polina2410/scout/backend/internal/minio"
)

// After store is created:
thumbCacheDir := filepath.Join(os.TempDir(), "scout-thumb-cache")
thumbCache, err := thumb.NewDiskCache(thumbCacheDir, cfg.ThumbCacheSizeMB*1024*1024)
if err != nil {
    log.Error("failed to create thumb cache", "error", err)
    os.Exit(1)
}

thumbSvc := thumb.New(store, thumbCache, log)  // store satisfies thumb.Downloader

mux.HandleFunc("GET /thumbnails/{photoId}", thumbSvc.Handle)
```

`store` is `*minioclient.Client` — it satisfies both `minioclient.Presigner` (for API routes) and `thumb.Downloader` (for thumbnails) without any cast.

---

## 10. Tests

### `internal/thumb/params_test.go`

| Test | Input | Assert |
|------|-------|--------|
| `TestParseParams_OK` | `w=400&dpr=2&fmt=webp`, valid UUID | `PxWidth==800`, no errors |
| `TestParseParams_DefaultDPRFmt` | `w=200`, valid UUID | `DPR==1`, `Fmt=="webp"`, `PxWidth==200` |
| `TestParseParams_MissingW` | no `w` param | error on field `"w"` |
| `TestParseParams_WZero` | `w=0` | error on field `"w"` |
| `TestParseParams_WTooLarge` | `w=9999` | error on field `"w"` |
| `TestParseParams_InvalidDPR` | `dpr=4` | error on field `"dpr"` |
| `TestParseParams_InvalidFmt` | `fmt=avif` | error on field `"fmt"` |
| `TestParseParams_PxWidthClamped` | `w=2560&dpr=3` | `PxWidth==2560` (clamped from 7680) |
| `TestParseParams_InvalidUUID` | `photoId="not-a-uuid"` | error on field `"photoId"` |

### `internal/thumb/cache_test.go`

| Test | Setup | Assert |
|------|-------|--------|
| `TestDiskCache_PutGet` | Put `"key"` → `[]byte("hello")`, Get `"key"` | bytes match |
| `TestDiskCache_Miss` | Get unknown key | returns `false` |
| `TestDiskCache_LRUEviction` | maxSize=10; Put `"a"`(6B), Put `"b"`(6B) | `"a"` evicted, `"b"` present |
| `TestDiskCache_UpdateMovesToFront` | Put `"a"`, Put `"b"`, Get `"a"`, Put `"c"`(forces eviction) | `"b"` evicted (LRU), `"a"` present |
| `TestDiskCache_ConcurrentAccess` | 20 goroutines doing Put/Get in parallel | no data race (run with `-race`) |

### `internal/thumb/service_test.go`

Use an `httptest.NewServer` with `thumbSvc.Handle` (no auth middleware for unit tests). Use a mock `Downloader` that returns a 1×1 white JPEG for success or a typed error for not-found.

A minimal valid 1×1 white JPEG can be generated in the test using `image/jpeg` encode into a buffer.

| Test | Setup | Assert |
|------|-------|--------|
| `TestHandle_CacheMiss_WebP` | empty cache, mock returns valid JPEG | 200, `Content-Type: image/webp`, `X-Cache: MISS` |
| `TestHandle_CacheMiss_JPEG` | empty cache, `fmt=jpeg`, mock returns valid JPEG | 200, `Content-Type: image/jpeg` |
| `TestHandle_CacheHit` | pre-seed cache with known bytes | 200, `X-Cache: HIT`, mock `GetOriginal` never called |
| `TestHandle_NotFound` | mock returns `ErrObjectNotFound` | 404, `code: "NotFound"` |
| `TestHandle_AtCapacity` | pre-fill `sem` with `maxConcurrent` tokens | 503, `Retry-After: 5` header present |
| `TestHandle_MissingW` | no `w` query param | 400, `code: "ValidationError"` |
| `TestHandle_MetricsCounted` | one cache hit, one cache miss | `Metrics().CacheHits==1`, `Metrics().CacheMisses==1` |
| `TestHandle_SecondRequestUsesCache` | two sequential identical requests | second has `X-Cache: HIT`, mock called only once |

---

## Acceptance criteria

- [ ] `GET /thumbnails/{photoId}?w=400&dpr=2&fmt=webp` returns a valid WebP image with pixel width 800
- [ ] `GET /thumbnails/{photoId}?w=400&fmt=jpeg` returns a valid JPEG image
- [ ] A second identical request returns `X-Cache: HIT` without calling MinIO again
- [ ] `GET /thumbnails/{photoId}` with missing `w` returns 400 `ValidationError` with `field: "w"`
- [ ] `GET /thumbnails/{photoId}` with `dpr=4` returns 400 `ValidationError` with `field: "dpr"`
- [ ] Returns 404 when the photo has not been seeded to MinIO
- [ ] With 4 generations in flight, a 5th concurrent request returns 503 with `Retry-After: 5`
- [ ] Identical concurrent requests share one MinIO download (verified by mock call count)
- [ ] Disk cache respects `THUMB_CACHE_SIZE_MB` — oldest entries are evicted when the limit is reached
- [ ] `/thumbnails/` is accessible without an `X-API-Key` header (auth bypassed)
- [ ] `go build ./...` and `go vet ./...` pass
- [ ] `go test ./internal/thumb/...` passes without a live MinIO (mock Downloader)
