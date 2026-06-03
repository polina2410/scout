package middleware

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// APIKeyAuth returns middleware that enforces X-API-Key on every request.
// Requests to /health bypass auth — that route is a liveness probe.
// Panics if key is empty — an empty key would grant access to all requests.
func APIKeyAuth(key string) func(http.Handler) http.Handler {
	if key == "" {
		panic("APIKeyAuth: empty API key — server cannot start with an open API")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			given := r.Header.Get("X-API-Key")
			if subtle.ConstantTimeCompare([]byte(given), []byte(key)) != 1 {
				writeUnauthorized(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	body := struct {
		RequestID string `json:"request_id"`
		Message   string `json:"message"`
		Code      string `json:"code"`
	}{
		RequestID: RequestIDFromContext(r.Context()),
		Message:   "missing or invalid API key",
		Code:      "AuthenticationRequired",
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	buf.WriteTo(w) //nolint:errcheck
}
