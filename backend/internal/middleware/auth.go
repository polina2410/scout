package middleware

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// APIKeyAuth returns middleware that enforces X-API-Key on every request.
// Requests to /health and /thumbnails/ bypass auth.
// Panics if key is empty — an empty key would grant access to all requests.
//
// Comparison uses SHA-256 hashes so both operands are always 32 bytes,
// preventing timing attacks that could leak the key length via early exit.
func APIKeyAuth(key string) func(http.Handler) http.Handler {
	if key == "" {
		panic("APIKeyAuth: empty API key — server cannot start with an open API")
	}
	keyHash := sha256.Sum256([]byte(key))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/thumbnails/") {
				next.ServeHTTP(w, r)
				return
			}
			givenHash := sha256.Sum256([]byte(r.Header.Get("X-API-Key")))
			if subtle.ConstantTimeCompare(givenHash[:], keyHash[:]) != 1 {
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
