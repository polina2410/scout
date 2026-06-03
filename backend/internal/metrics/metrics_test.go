package metrics

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCollector_StatusSplit(t *testing.T) {
	c := NewCollector()
	c.Observe(10, 200)
	c.Observe(10, 404)
	c.Observe(10, 500)

	s := c.Snapshot()
	if s.Total != 3 {
		t.Errorf("Total: got %d, want 3", s.Total)
	}
	if s.Total2xx != 1 {
		t.Errorf("Total2xx: got %d, want 1", s.Total2xx)
	}
	if s.Total4xx != 1 {
		t.Errorf("Total4xx: got %d, want 1", s.Total4xx)
	}
	if s.Total5xx != 1 {
		t.Errorf("Total5xx: got %d, want 1", s.Total5xx)
	}
}

func TestCollector_RateAndErrorRate(t *testing.T) {
	c := NewCollector()
	// Set startTime 1s in the past so UptimeSec > 0 immediately.
	c.startTime = time.Now().Add(-time.Second)

	for i := 0; i < 8; i++ {
		c.Observe(10, 200)
	}
	for i := 0; i < 2; i++ {
		c.Observe(10, 500)
	}

	s := c.Snapshot()
	if s.RequestsPerSec <= 0 {
		t.Errorf("RequestsPerSec: got %v, want > 0", s.RequestsPerSec)
	}
	const wantErrorRate = 0.2
	if math.Abs(s.ErrorRate-wantErrorRate) > 1e-9 {
		t.Errorf("ErrorRate: got %v, want %v", s.ErrorRate, wantErrorRate)
	}
}

func TestCollector_LatencyP50(t *testing.T) {
	c := NewCollector()
	for i := int64(1); i <= 10; i++ {
		c.Observe(i, 200)
	}

	s := c.Snapshot()
	if s.LatencyP50Ms > 5 {
		t.Errorf("LatencyP50Ms: got %v, want <= 5 (within one bucket of true median)", s.LatencyP50Ms)
	}
}

func TestCollector_LatencyP95(t *testing.T) {
	c := NewCollector()
	for i := 0; i < 19; i++ {
		c.Observe(1, 200)
	}
	c.Observe(500, 200)

	s := c.Snapshot()
	if s.LatencyP95Ms > 500 {
		t.Errorf("LatencyP95Ms: got %v, want <= 500", s.LatencyP95Ms)
	}
	if s.LatencyP50Ms > 5 {
		t.Errorf("LatencyP50Ms: got %v, want <= 5", s.LatencyP50Ms)
	}
}

func TestCollector_MeanLatency(t *testing.T) {
	c := NewCollector()
	c.Observe(10, 200)
	c.Observe(30, 200)

	s := c.Snapshot()
	const wantMean = 20.0
	if math.Abs(s.LatencyMeanMs-wantMean) > 0.001 {
		t.Errorf("LatencyMeanMs: got %v, want %v", s.LatencyMeanMs, wantMean)
	}
}

func TestCollector_ZeroState(t *testing.T) {
	c := NewCollector()
	s := c.Snapshot()

	if s.Total != 0 {
		t.Errorf("Total: got %d, want 0", s.Total)
	}
	if math.IsNaN(s.RequestsPerSec) {
		t.Error("RequestsPerSec is NaN, want 0")
	}
	if math.IsNaN(s.ErrorRate) {
		t.Error("ErrorRate is NaN, want 0")
	}
	if math.IsNaN(s.LatencyMeanMs) {
		t.Error("LatencyMeanMs is NaN, want 0")
	}
	if math.IsNaN(s.LatencyP50Ms) {
		t.Error("LatencyP50Ms is NaN, want 0")
	}
	if math.IsNaN(s.LatencyP95Ms) {
		t.Error("LatencyP95Ms is NaN, want 0")
	}
	if s.RequestsPerSec != 0 {
		t.Errorf("RequestsPerSec: got %v, want 0 (no requests)", s.RequestsPerSec)
	}
	if s.ErrorRate != 0 {
		t.Errorf("ErrorRate: got %v, want 0 (no requests)", s.ErrorRate)
	}
}

func TestMiddleware_ObservesRequest(t *testing.T) {
	c := NewCollector()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mw := Middleware(c)(handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	s := c.Snapshot()
	if s.Total != 1 {
		t.Errorf("Total: got %d, want 1", s.Total)
	}
	if s.Total2xx != 1 {
		t.Errorf("Total2xx: got %d, want 1", s.Total2xx)
	}
}
