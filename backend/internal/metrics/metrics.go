package metrics

import (
	"math"
	"net/http"
	"sync"
	"time"
)

var bucketBoundsMs = [...]int64{1, 5, 10, 25, 50, 100, 250, 500, 1000}

const numBuckets = len(bucketBoundsMs) + 1

// Collector aggregates per-request metrics.
type Collector struct {
	mu        sync.Mutex
	total     int64
	total2xx  int64
	total4xx  int64
	total5xx  int64
	totalNs   int64
	buckets   [numBuckets]int64
	startTime time.Time
}

// NewCollector returns a Collector with its start time set to now.
func NewCollector() *Collector {
	return &Collector{startTime: time.Now()}
}

// Observe records one request with the given duration (in milliseconds) and HTTP status code.
func (c *Collector) Observe(durationMs int64, statusCode int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.total++
	switch statusCode / 100 {
	case 2:
		c.total2xx++
	case 4:
		c.total4xx++
	case 5:
		c.total5xx++
	}
	c.totalNs += durationMs * int64(time.Millisecond)

	idx := numBuckets - 1
	for i, bound := range bucketBoundsMs {
		if bound >= durationMs {
			idx = i
			break
		}
	}
	c.buckets[idx]++
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
	c.mu.Lock()
	defer c.mu.Unlock()

	uptime := time.Since(c.startTime).Seconds()

	var rps float64
	if uptime > 0 {
		rps = float64(c.total) / uptime
	}

	var errorRate float64
	if c.total > 0 {
		errorRate = float64(c.total5xx) / float64(c.total)
	}

	var meanMs float64
	if c.total > 0 {
		meanMs = float64(c.totalNs) / float64(c.total) / float64(time.Millisecond)
	}

	return Snapshot{
		Total:          c.total,
		Total2xx:       c.total2xx,
		Total4xx:       c.total4xx,
		Total5xx:       c.total5xx,
		UptimeSec:      uptime,
		RequestsPerSec: rps,
		ErrorRate:      errorRate,
		LatencyMeanMs:  meanMs,
		LatencyP50Ms:   percentileMs(c.buckets, c.total, 0.50),
		LatencyP95Ms:   percentileMs(c.buckets, c.total, 0.95),
	}
}

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
