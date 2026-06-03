package handler_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/polina2410/scout/backend/internal/handler"
	metricspkg "github.com/polina2410/scout/backend/internal/metrics"
)

func TestMetricsHandler_OK(t *testing.T) {
	col := metricspkg.NewCollector()
	col.Observe(10, 200)
	col.Observe(10, 500)
	// Ensure uptime > 0 for requests_per_sec.
	time.Sleep(time.Millisecond)

	thumb := func() handler.ThumbSnapshot {
		return handler.ThumbSnapshot{
			CacheHits:    10,
			CacheMisses:  2,
			CacheEntries: 5,
			CacheBytes:   1024,
			GenOK:        5,
			GenErr:       0,
			GenTotalNs:   5 * 250 * int64(time.Millisecond),
			GenP95Ms:     250.0,
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.MetricsHandler(col, thumb).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	requests, ok := body["requests"].(map[string]any)
	if !ok {
		t.Fatal("missing requests section")
	}
	if total := requests["total"].(float64); total != 2 {
		t.Errorf("requests.total: got %v, want 2", total)
	}
	if errRate := requests["error_rate"].(float64); math.Abs(errRate-0.5) > 1e-9 {
		t.Errorf("requests.error_rate: got %v, want 0.5", errRate)
	}
	if rps := requests["requests_per_sec"].(float64); rps <= 0 {
		t.Errorf("requests.requests_per_sec: got %v, want > 0", rps)
	}

	thumbnails, ok := body["thumbnails"].(map[string]any)
	if !ok {
		t.Fatal("missing thumbnails section")
	}
	hitRate := thumbnails["cache_hit_rate"].(float64)
	const wantHitRate = 10.0 / 12.0
	if math.Abs(hitRate-wantHitRate) > 0.001 {
		t.Errorf("thumbnails.cache_hit_rate: got %v, want ~0.833", hitRate)
	}
	if p95 := thumbnails["gen_p95_ms"].(float64); p95 != 250.0 {
		t.Errorf("thumbnails.gen_p95_ms: got %v, want 250.0", p95)
	}
}

func TestMetricsHandler_ZeroState(t *testing.T) {
	col := metricspkg.NewCollector()
	thumb := func() handler.ThumbSnapshot { return handler.ThumbSnapshot{} }

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.MetricsHandler(col, thumb).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	check := func(section map[string]any, field string) {
		t.Helper()
		v, ok := section[field].(float64)
		if !ok {
			t.Errorf("%s: missing or not a number", field)
			return
		}
		if math.IsNaN(v) {
			t.Errorf("%s: got NaN, want 0", field)
		}
		if v != 0 {
			t.Errorf("%s: got %v, want 0", field, v)
		}
	}

	requests := body["requests"].(map[string]any)
	check(requests, "requests_per_sec")
	check(requests, "error_rate")
	check(requests, "latency_mean_ms")
	check(requests, "latency_p50_ms")
	check(requests, "latency_p95_ms")

	thumbnails := body["thumbnails"].(map[string]any)
	check(thumbnails, "cache_hit_rate")
	check(thumbnails, "gen_mean_ms")
	check(thumbnails, "gen_p95_ms")
}
