# Spec 07 — /metrics Endpoint

**Plan ref:** Phase 4, Step 7  
**Goal:** Implement `GET /metrics` that returns request totals, latency percentiles (p50/p95), error counts, and thumbnail cache/generation statistics as a single JSON response — all counters maintained in-process with no external dependency.

---

## 1. New package — `internal/metrics/metrics.go`

All request-level counters and the latency histogram live here. No other package is imported except `sync` and `time`.

```go
package metrics

import (
    "math"
    "sync"
    "time"
)

// bucketBoundsMs are the upper bounds (inclusive, in milliseconds) of the fixed
// latency histogram buckets. Requests slower than the last bound fall into the
// implicit +Inf bucket at index len(bucketBoundsMs).
var bucketBoundsMs = [...]int64{1, 5, 10, 25, 50, 100, 250, 500, 1000}

const numBuckets = len(bucketBoundsMs) + 1 // last bucket is +Inf

// Collector aggregates per-request metrics.
// All exported and unexported fields are protected by mu; never access them directly.
type Collector struct {
    mu        sync.Mutex
    total     int64
    total2xx  int64
    total4xx  int64
    total5xx  int64
    totalNs   int64
    buckets   [numBuckets]int64 // request counts per latency bucket
    startTime time.Time
}

// NewCollector returns a Collector with its start time set to now.
func NewCollector() *Collector {
    return &Collector{startTime: time.Now()}
}
```

**`Observe(durationMs int64, statusCode int)`**

Called once per request by the middleware. All writes are under `mu`.

1. `c.total++`
2. Increment `c.total2xx`, `c.total4xx`, or `c.total5xx` depending on `statusCode / 100`
3. `c.totalNs += durationMs * int64(time.Millisecond)`
4. Find the first bucket index `i` where `bucketBoundsMs[i] >= durationMs`; if none, use the +Inf index. Increment `c.buckets[i]`.

**`Snapshot() Snapshot`**

Locks, copies all fields, computes derived values, unlocks.

```go
// Snapshot is a point-in-time view of collected request metrics.
type Snapshot struct {
    Total          int64
    Total2xx       int64
    Total4xx       int64
    Total5xx       int64
    UptimeSec      float64
    RequestsPerSec float64 // Total / UptimeSec; 0 when uptime is zero
    ErrorRate      float64 // Total5xx / Total; 0 when Total is zero
    LatencyMeanMs  float64
    LatencyP50Ms   float64
    LatencyP95Ms   float64
}
```

**Percentile computation** (derived from histogram, called inside lock while values are copied):

```go
func percentileMs(buckets [numBuckets]int64, total int64, p float64) float64 {
    if total == 0 {
        return 0
    }
    target := int64(math.Ceil(p * float64(total)))
    var cum int64
    for i, count := range buckets {
        cum += count
        if cum >= target {
            if i < len(bucketBoundsMs) {
                return float64(bucketBoundsMs[i])
            }
            // +Inf bucket: return the last finite bound as a floor value
            return float64(bucketBoundsMs[len(bucketBoundsMs)-1])
        }
    }
    return 0
}
```

`LatencyMeanMs = float64(totalNs) / float64(total) / float64(time.Millisecond)` — zero when `total == 0`.  
`UptimeSec = time.Since(startTime).Seconds()`  
`RequestsPerSec = float64(total) / UptimeSec` — zero when `UptimeSec == 0`.  
`ErrorRate = float64(total5xx) / float64(total)` — zero when `total == 0`.

---

## 2. Middleware — `internal/metrics/metrics.go`

```go
// Middleware returns an HTTP middleware that calls Observe after every request.
func Middleware(c *Collector) func(http.Handler) http.Handler
```

Uses an unexported `responseWriter` wrapper (same pattern as `middleware.CorrelationID`) to capture the final status code after `next.ServeHTTP` returns. Records `time.Since(start).Milliseconds()` as the duration.

`Observe` is called with the captured status code and duration. If no `WriteHeader` was called (the handler wrote a body without setting status), assume 200.

---

## 3. Handler type — `internal/handler/metrics.go`

To avoid an import cycle (`thumb` already imports `handler`), the metrics handler receives thumbnail data through a callback rather than importing `thumb` directly.

```go
// ThumbSnapshot carries the thumbnail counters needed by the metrics response.
// It is populated in main.go from thumb.Service.Metrics() + thumb.DiskCache.Stats().
type ThumbSnapshot struct {
    CacheHits    int64
    CacheMisses  int64
    CacheEntries int
    CacheBytes   int64
    GenOK        int64
    GenErr       int64
    GenTotalNs   int64
    GenP95Ms     float64 // pre-computed by thumb.Service.Metrics() from its generation histogram
}

// MetricsHandler returns an HTTP handler for GET /metrics.
// col provides request-level metrics; thumbMetrics is called on each request to get a fresh snapshot.
func MetricsHandler(col *metricspkg.Collector, thumbMetrics func() ThumbSnapshot) http.HandlerFunc
```

Import alias: `metricspkg "github.com/polina2410/scout/backend/internal/metrics"`.

---

## 4. Response shape

`MetricsHandler` serialises the following structs via `WriteJSON`:

```go
type metricsResponse struct {
    Requests   requestMetrics   `json:"requests"`
    Thumbnails thumbnailMetrics `json:"thumbnails"`
}

type requestMetrics struct {
    Total          int64   `json:"total"`
    Total2xx       int64   `json:"total_2xx"`
    Total4xx       int64   `json:"total_4xx"`
    Total5xx       int64   `json:"total_5xx"`
    UptimeSec      float64 `json:"uptime_seconds"`
    RequestsPerSec float64 `json:"requests_per_sec"`  // total / uptime_seconds; 0 when uptime is zero
    ErrorRate      float64 `json:"error_rate"`         // total_5xx / total; 0 when total is zero
    LatencyMeanMs  float64 `json:"latency_mean_ms"`
    LatencyP50Ms   float64 `json:"latency_p50_ms"`
    LatencyP95Ms   float64 `json:"latency_p95_ms"`
}

type thumbnailMetrics struct {
    CacheHits    int64   `json:"cache_hits"`
    CacheMisses  int64   `json:"cache_misses"`
    CacheHitRate float64 `json:"cache_hit_rate"`  // hits / (hits + misses); 0 when both are 0
    CacheEntries int     `json:"cache_entries"`
    CacheBytes   int64   `json:"cache_bytes"`
    GenOK        int64   `json:"gen_ok"`
    GenErr       int64   `json:"gen_err"`
    GenMeanMs    float64 `json:"gen_mean_ms"`  // GenTotalNs / GenOK / 1e6; 0 when GenOK == 0
    GenP95Ms     float64 `json:"gen_p95_ms"`   // p95 of generation time from thumb.Service histogram
}
```

All struct types are unexported — only the JSON field names are part of the public contract.

**Example response:**

```json
{
  "requests": {
    "total": 1234,
    "total_2xx": 1100,
    "total_4xx": 120,
    "total_5xx": 14,
    "uptime_seconds": 3600.0,
    "requests_per_sec": 0.343,
    "error_rate": 0.011,
    "latency_mean_ms": 18.3,
    "latency_p50_ms": 10.0,
    "latency_p95_ms": 100.0
  },
  "thumbnails": {
    "cache_hits": 890,
    "cache_misses": 123,
    "cache_hit_rate": 0.878,
    "cache_entries": 45,
    "cache_bytes": 123456789,
    "gen_ok": 123,
    "gen_err": 2,
    "gen_mean_ms": 234.5,
    "gen_p95_ms": 500.0
  }
}
```

---

## 5. Carry-forward — generation time histogram in `internal/thumb/`

`thumb.Service` currently tracks only cumulative `genNs`. To expose `gen_p95_ms`, the service needs a fixed-bucket histogram of individual generation durations. Make these changes as part of this step:

**New constants in `internal/thumb/service.go`:**

```go
// genBucketBoundsMs are the upper bounds (inclusive, ms) for generation time buckets.
// Bounds are higher than the request latency histogram because generation involves
// a MinIO fetch + JPEG decode + resize + encode — typically 100ms–2000ms.
var genBucketBoundsMs = [...]int64{50, 100, 250, 500, 750, 1000, 2000, 3000, 5000}

const numGenBuckets = len(genBucketBoundsMs) + 1 // +Inf bucket at end
```

**Updated `Service` struct** — replace `genNs atomic.Int64` with:

```go
genNs      atomic.Int64
genBuckets [numGenBuckets]atomic.Int64
```

**Updated `generate()`** — after recording `s.genNs.Add(...)`:

```go
durationMs := time.Since(t0).Milliseconds()
s.genNs.Add(time.Since(t0).Nanoseconds())  // existing line (use same t0)
bucketIdx := numGenBuckets - 1             // default: +Inf bucket
for i, bound := range genBucketBoundsMs {
    if durationMs <= bound {
        bucketIdx = i
        break
    }
}
s.genBuckets[bucketIdx].Add(1)
s.genOK.Add(1)
```

**Updated `ThumbMetrics`:**

```go
type ThumbMetrics struct {
    CacheHits   int64
    CacheMisses int64
    GenOK       int64
    GenErr      int64
    GenTotalNs  int64
    GenP95Ms    float64 // computed from genBuckets in Metrics()
}
```

**Updated `Metrics()`** — add `GenP95Ms` computation (mirrors `percentileMs` in the `metrics` package):

```go
func genPercentileMs(buckets [numGenBuckets]int64, total int64, p float64) float64 {
    if total == 0 {
        return 0
    }
    target := int64(math.Ceil(p * float64(total)))
    var cum int64
    for i, count := range buckets {
        cum += count
        if cum >= target {
            if i < len(genBucketBoundsMs) {
                return float64(genBucketBoundsMs[i])
            }
            return float64(genBucketBoundsMs[len(genBucketBoundsMs)-1])
        }
    }
    return 0
}
```

Import `math` in `service.go`. Call `genPercentileMs` from `Metrics()` after loading each bucket's value into a local `[numGenBuckets]int64` array (one `Load()` call per bucket under no lock — acceptable given these are independent atomics and the snapshot need not be transactionally consistent).

**Updated wiring in `main.go`** — add `GenP95Ms` to the `ThumbSnapshot` closure:

```go
return handler.ThumbSnapshot{
    // ... existing fields ...
    GenP95Ms: m.GenP95Ms,
}
```

**Updated service test** — add one case to `TestHandle_MetricsCounted`:

| Assert | Value |
|--------|-------|
| After one successful generation | `svc.Metrics().GenP95Ms > 0` |

---

## 6. Auth

No change to `middleware/auth.go`. `GET /metrics` goes through `APIKeyAuth` unchanged — monitoring systems can set request headers, so there is no technical reason to bypass auth here.

---

## 7. Wiring — `main.go`

```go
import (
    metricspkg "github.com/polina2410/scout/backend/internal/metrics"
)

// After thumbSvc and thumbCache are created:
collector := metricspkg.NewCollector()

// Register /metrics before wrapping the mux with middleware.
mux.HandleFunc("GET /metrics", handler.MetricsHandler(collector, func() handler.ThumbSnapshot {
    m := thumbSvc.Metrics()
    entries, bytes := thumbCache.Stats()
    return handler.ThumbSnapshot{
        CacheHits:    m.CacheHits,
        CacheMisses:  m.CacheMisses,
        CacheEntries: entries,
        CacheBytes:   bytes,
        GenOK:        m.GenOK,
        GenErr:       m.GenErr,
        GenTotalNs:   m.GenTotalNs,
        GenP95Ms:     m.GenP95Ms,
    }
}))

// Middleware stack (outermost first):
var h http.Handler = mux
h = middleware.APIKeyAuth(cfg.APIKey)(h)
h = metricspkg.Middleware(collector)(h)  // counts all requests including 401s
h = middleware.CorrelationID(log)(h)
```

`GET /metrics` is counted by `MetricsMiddleware` like any other route — querying metrics is itself a request.

---

## 8. Tests — `internal/metrics/metrics_test.go`

| Test | Setup | Assert |
|------|-------|--------|
| `TestCollector_StatusSplit` | Observe 3 requests: 200, 404, 500 | `total==3`, `total_2xx==1`, `total_4xx==1`, `total_5xx==1` |
| `TestCollector_RateAndErrorRate` | Observe 10 requests: 8×200, 2×500; snapshot taken >0s after start | `requests_per_sec > 0`, `error_rate == 0.2` |
| `TestCollector_LatencyP50` | Observe 10 requests, durations 1–10 ms | `p50 ≤ 5` (within one bucket of true median) |
| `TestCollector_LatencyP95` | Observe 20 requests, 19 at 1 ms, 1 at 500 ms | `p95 ≤ 500` and `p50 ≤ 5` |
| `TestCollector_MeanLatency` | Observe 2 requests: 10 ms and 30 ms | `mean == 20.0 ms` (within float tolerance) |
| `TestCollector_ZeroState` | Fresh Collector, no observations | `Snapshot()` returns all-zero fields, no NaN, no panic |
| `TestMiddleware_ObservesRequest` | Wrap a handler that returns 201; call it once | `Snapshot().Total == 1`, `Total2xx == 1` |

**Handler test — `internal/handler/metrics_test.go`:**

| Test | Setup | Assert |
|------|-------|--------|
| `TestMetricsHandler_OK` | Collector with 2 observed requests (200 + 500); thumbMetrics returns `{CacheHits:10, CacheMisses:2, GenOK:5, GenP95Ms:250.0, ...}` | 200, `requests.total==2`, `requests.error_rate==0.5`, `requests.requests_per_sec>0`, `thumbnails.cache_hit_rate≈0.833`, `thumbnails.gen_p95_ms==250.0` |
| `TestMetricsHandler_ZeroState` | Fresh Collector; thumbMetrics returns all zeros | 200, all numeric fields are `0` or `0.0` — no NaN anywhere, no panic |

Use `httptest.NewRecorder` + `httptest.NewRequest`. No auth middleware in unit tests — call `MetricsHandler` directly.

---

## Acceptance criteria

- [ ] `GET /metrics` with valid `X-API-Key` returns 200 and valid JSON
- [ ] `GET /metrics` without `X-API-Key` returns 401 (goes through standard auth)
- [ ] `requests.total` increments by 1 after each request to any route (including `/health`)
- [ ] `requests.total_5xx` increments after a handler returns a 5xx status
- [ ] `requests.requests_per_sec` equals `total / uptime_seconds` and is positive after at least one request
- [ ] `requests.error_rate` equals `total_5xx / total`; exactly `0.0` when no requests have been made
- [ ] `requests.latency_p50_ms` and `latency_p95_ms` are non-negative and non-NaN
- [ ] `thumbnails.cache_hits` and `thumbnails.cache_misses` match `thumb.Service.Metrics()` values
- [ ] `thumbnails.cache_hit_rate` is `hits / (hits + misses)` when non-zero, exactly `0` when both are zero
- [ ] `thumbnails.gen_mean_ms` and `gen_p95_ms` are exactly `0` (not NaN) before any thumbnail is generated
- [ ] `thumbnails.gen_p95_ms` is positive after at least one successful thumbnail generation
- [ ] `go build ./...` and `go vet ./...` pass
- [ ] `go test ./internal/metrics/... ./internal/handler/... ./internal/thumb/...` passes
