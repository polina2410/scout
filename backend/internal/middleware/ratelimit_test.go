package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowsBurstThenBlocks(t *testing.T) {
	rl := NewRateLimiter(1, 3) // 3 burst, refill 1/sec
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d within burst should be allowed", i+1)
		}
	}
	if rl.allow("1.2.3.4") {
		t.Error("4th request should be blocked once burst is exhausted")
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 burst, refill 1/sec
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	if !rl.allow("c") {
		t.Fatal("first request should be allowed")
	}
	if rl.allow("c") {
		t.Fatal("second immediate request should be blocked")
	}

	now = now.Add(time.Second) // one token refills
	if !rl.allow("c") {
		t.Error("request after 1s refill should be allowed")
	}
}

func TestRateLimiter_IsolatesClients(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	if !rl.allow("a") {
		t.Fatal("client a first request should be allowed")
	}
	// A different client must not be affected by a's consumed token.
	if !rl.allow("b") {
		t.Error("client b should have its own bucket")
	}
}

func TestRateLimiter_Middleware429(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := rl.Middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/thumbnails/x", nil)
	req.RemoteAddr = "9.9.9.9:1234"

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Errorf("first request: got %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: got %d, want 429", rec2.Code)
	}
	if ra := rec2.Header().Get("Retry-After"); ra == "" {
		t.Error("429 response should set Retry-After header")
	}
}
