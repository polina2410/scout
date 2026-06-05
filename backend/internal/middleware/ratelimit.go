package middleware

import (
	"bytes"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// rateLimitRetryAfterSec is the Retry-After hint (seconds) sent with 429s.
	rateLimitRetryAfterSec = 1
	// rateLimitIdleTTL is how long an idle client bucket is kept before eviction.
	rateLimitIdleTTL = 10 * time.Minute
)

// tokenBucket is a single client's token-bucket state.
type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-client (by IP) token-bucket rate limiter. It bounds how
// fast any single client can hit a protected endpoint, preventing one caller
// from monopolizing a shared, unauthenticated resource — here, the thumbnail
// generation semaphore, which an unthrottled client could otherwise keep
// saturated, starving legitimate gallery users with 503s.
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	rate       float64 // tokens refilled per second
	burst      float64 // maximum tokens, and the initial allowance
	trustProxy bool    // honor X-Forwarded-For / X-Real-IP for the client IP
	now        func() time.Time
}

// NewRateLimiter creates a limiter allowing burst requests immediately and
// refilling at ratePerSec tokens/second per client IP. When trustProxy is true
// the client IP is read from proxy forwarding headers (only safe behind a
// trusted reverse proxy); otherwise it is the direct connection's RemoteAddr.
func NewRateLimiter(ratePerSec, burst float64, trustProxy bool) *RateLimiter {
	return &RateLimiter{
		buckets:    make(map[string]*tokenBucket),
		rate:       ratePerSec,
		burst:      burst,
		trustProxy: trustProxy,
		now:        time.Now,
	}
}

// allow reports whether a request from client is permitted, consuming a token.
func (rl *RateLimiter) allow(client string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	b, ok := rl.buckets[client]
	if !ok {
		// Evict stale buckets whenever a new client appears, bounding memory
		// without a background goroutine.
		rl.evictStale(now)
		b = &tokenBucket{tokens: rl.burst, lastSeen: now}
		rl.buckets[client] = b
	} else {
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens = math.Min(rl.burst, b.tokens+elapsed*rl.rate)
		b.lastSeen = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) evictStale(now time.Time) {
	for k, b := range rl.buckets {
		if now.Sub(b.lastSeen) > rateLimitIdleTTL {
			delete(rl.buckets, k)
		}
	}
}

// Middleware enforces the per-client rate limit, returning 429 when exceeded.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(rl.clientIP(r)) {
			writeTooManyRequests(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP determines the rate-limit key for a request. Behind a trusted proxy
// it prefers the left-most X-Forwarded-For entry (the original client), then
// X-Real-IP; otherwise — and as a fallback — it uses the direct connection's
// RemoteAddr without its port.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	if rl.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// The list is "client, proxy1, proxy2"; the first entry is the origin.
			first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
			if first != "" {
				return first
			}
		}
		if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
			return xrip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeTooManyRequests(w http.ResponseWriter, r *http.Request) {
	body := struct {
		RequestID string `json:"request_id"`
		Message   string `json:"message"`
		Code      string `json:"code"`
	}{
		RequestID: RequestIDFromContext(r.Context()),
		Message:   "rate limit exceeded, retry shortly",
		Code:      "TooManyRequests",
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(rateLimitRetryAfterSec))
	w.WriteHeader(http.StatusTooManyRequests)
	buf.WriteTo(w) //nolint:errcheck
}
