package metrics

import (
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var bucketBoundsMs = [...]int64{1, 5, 10, 25, 50, 100, 250, 500, 1000}

const numBuckets = len(bucketBoundsMs) + 1

// Collector aggregates per-request metrics. The scalar counters are atomics so
// the hot Observe path stays lock-free; the mutex guards only the histogram
// buckets, which have no atomic array equivalent.
type Collector struct {
	total    atomic.Int64
	total2xx atomic.Int64
	total4xx atomic.Int64
	total5xx atomic.Int64
	totalNs  atomic.Int64

	mu        sync.Mutex // guards buckets only
	buckets   [numBuckets]int64
	startTime time.Time
}

// NewCollector returns a Collector with its start time set to now.
func NewCollector() *Collector {
	return &Collector{startTime: time.Now()}
}

// Observe records one request with the given duration (in milliseconds) and HTTP status code.
func (c *Collector) Observe(durationMs int64, statusCode int) {
	c.total.Add(1)
	switch statusCode / 100 {
	case 2:
		c.total2xx.Add(1)
	case 4:
		c.total4xx.Add(1)
	case 5:
		c.total5xx.Add(1)
	}
	c.totalNs.Add(durationMs * int64(time.Millisecond))

	idx := numBuckets - 1
	for i, bound := range bucketBoundsMs {
		if bound >= durationMs {
			idx = i
			break
		}
	}
	c.mu.Lock()
	c.buckets[idx]++
	c.mu.Unlock()
}

// Snapshot is a point-in-time view of collected request metrics.
type Snapshot struct {
	Total          int64
	Total2xx       int64
	Total4xx       int64
	Total5xx       int64
	UptimeSec      float64
	RequestsPerSec float64
	ErrorRate      float64
	LatencyMeanMs  float64
	LatencyP50Ms   float64
	LatencyP95Ms   float64
}

// Snapshot returns a point-in-time copy of the collected metrics with derived fields computed.
func (c *Collector) Snapshot() Snapshot {
	total := c.total.Load()
	total2xx := c.total2xx.Load()
	total4xx := c.total4xx.Load()
	total5xx := c.total5xx.Load()
	totalNs := c.totalNs.Load()

	c.mu.Lock()
	buckets := c.buckets
	c.mu.Unlock()

	uptime := time.Since(c.startTime).Seconds()

	var rps float64
	if uptime > 0 {
		rps = float64(total) / uptime
	}

	var errorRate float64
	if total > 0 {
		errorRate = float64(total5xx) / float64(total)
	}

	var meanMs float64
	if total > 0 {
		meanMs = float64(totalNs) / float64(total) / float64(time.Millisecond)
	}

	return Snapshot{
		Total:          total,
		Total2xx:       total2xx,
		Total4xx:       total4xx,
		Total5xx:       total5xx,
		UptimeSec:      uptime,
		RequestsPerSec: rps,
		ErrorRate:      errorRate,
		LatencyMeanMs:  meanMs,
		LatencyP50Ms:   percentileMs(buckets, 0.50),
		LatencyP95Ms:   percentileMs(buckets, 0.95),
	}
}

// percentileMs derives its total from the buckets themselves so the result is
// always self-consistent, even if the atomic request counter has raced ahead of
// a not-yet-recorded bucket increment.
func percentileMs(buckets [numBuckets]int64, p float64) float64 {
	var total int64
	for _, count := range buckets {
		total += count
	}
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
			return float64(bucketBoundsMs[len(bucketBoundsMs)-1])
		}
	}
	return 0
}

// responseWriter wraps http.ResponseWriter to capture the written status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// Middleware returns an HTTP middleware that calls Observe after every request.
func Middleware(c *Collector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			c.Observe(time.Since(start).Milliseconds(), status)
		})
	}
}
