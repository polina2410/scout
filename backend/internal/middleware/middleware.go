package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// contextKey is an unexported type for context keys in this package.
// Using a named type prevents collisions with keys from other packages.
type contextKey int

const requestIDKey contextKey = 0

// RequestIDFromContext retrieves the request ID stored by CorrelationID.
// Returns "" if not set.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// CorrelationID is an HTTP middleware that assigns a request ID to every
// request, sets X-Request-ID on the response, and logs one access line.
func CorrelationID(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				var buf [12]byte
				if _, err := rand.Read(buf[:]); err != nil {
					logger.Warn("crypto/rand unavailable, using fallback request id", "error", err)
					id = "rand-err"
				} else {
					id = hex.EncodeToString(buf[:])
				}
			}

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", id)

			rw := &responseWriter{ResponseWriter: w}
			start := time.Now()

			next.ServeHTTP(rw, r)

			status := rw.status
			if !rw.wrote {
				status = http.StatusOK
			}

			logger.Info("request",
				"request_id", id,
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wrote {
		rw.status = code
		rw.wrote = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.status = http.StatusOK
		rw.wrote = true
	}
	return rw.ResponseWriter.Write(b)
}
