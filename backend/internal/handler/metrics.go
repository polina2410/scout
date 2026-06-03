package handler

import (
	"net/http"

	metricspkg "github.com/polina2410/scout/backend/internal/metrics"
)

// ThumbSnapshot carries the thumbnail counters needed by the metrics response.
// Populated in main.go from thumb.Service.Metrics() + thumb.DiskCache.Stats().
type ThumbSnapshot struct {
	CacheHits    int64
	CacheMisses  int64
	CacheEntries int
	CacheBytes   int64
	GenOK        int64
	GenErr       int64
	GenTotalNs   int64
	GenP95Ms     float64
}

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
	RequestsPerSec float64 `json:"requests_per_sec"`
	ErrorRate      float64 `json:"error_rate"`
	LatencyMeanMs  float64 `json:"latency_mean_ms"`
	LatencyP50Ms   float64 `json:"latency_p50_ms"`
	LatencyP95Ms   float64 `json:"latency_p95_ms"`
}

type thumbnailMetrics struct {
	CacheHits    int64   `json:"cache_hits"`
	CacheMisses  int64   `json:"cache_misses"`
	CacheHitRate float64 `json:"cache_hit_rate"`
	CacheEntries int     `json:"cache_entries"`
	CacheBytes   int64   `json:"cache_bytes"`
	GenOK        int64   `json:"gen_ok"`
	GenErr       int64   `json:"gen_err"`
	GenMeanMs    float64 `json:"gen_mean_ms"`
	GenP95Ms     float64 `json:"gen_p95_ms"`
}

// MetricsHandler returns an HTTP handler for GET /metrics.
func MetricsHandler(col *metricspkg.Collector, thumbMetrics func() ThumbSnapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := col.Snapshot()
		th := thumbMetrics()

		var cacheHitRate float64
		total := th.CacheHits + th.CacheMisses
		if total > 0 {
			cacheHitRate = float64(th.CacheHits) / float64(total)
		}

		var genMeanMs float64
		if th.GenOK > 0 {
			genMeanMs = float64(th.GenTotalNs) / float64(th.GenOK) / 1e6
		}

		WriteJSON(w, http.StatusOK, metricsResponse{
			Requests: requestMetrics{
				Total:          snap.Total,
				Total2xx:       snap.Total2xx,
				Total4xx:       snap.Total4xx,
				Total5xx:       snap.Total5xx,
				UptimeSec:      snap.UptimeSec,
				RequestsPerSec: snap.RequestsPerSec,
				ErrorRate:      snap.ErrorRate,
				LatencyMeanMs:  snap.LatencyMeanMs,
				LatencyP50Ms:   snap.LatencyP50Ms,
				LatencyP95Ms:   snap.LatencyP95Ms,
			},
			Thumbnails: thumbnailMetrics{
				CacheHits:    th.CacheHits,
				CacheMisses:  th.CacheMisses,
				CacheHitRate: cacheHitRate,
				CacheEntries: th.CacheEntries,
				CacheBytes:   th.CacheBytes,
				GenOK:        th.GenOK,
				GenErr:       th.GenErr,
				GenMeanMs:    genMeanMs,
				GenP95Ms:     th.GenP95Ms,
			},
		})
	}
}
