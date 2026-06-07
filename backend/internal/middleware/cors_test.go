package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_ExposesXCacheForLocalhostOrigin(t *testing.T) {
	h := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/thumbnails/abc", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Cache" {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, "X-Cache")
	}
}

func TestCORS_NoHeadersForNonLocalhostOrigin(t *testing.T) {
	h := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/thumbnails/abc", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "" {
		t.Errorf("expected no Expose-Headers for non-localhost origin, got %q", got)
	}
}
