package webhook

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// tokenBucket is a simple token-bucket rate limiter.
// Tokens refill at a fixed rate; each request consumes one token.
// Thread-safe.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	maxBurst float64
	rate     float64 // tokens per second
	lastFill time.Time
}

func newTokenBucket(perMinute int) *tokenBucket {
	max := float64(perMinute)
	return &tokenBucket{
		tokens:   max,
		maxBurst: max,
		rate:     max / 60.0,
		lastFill: time.Now(),
	}
}

// allow returns true if a request may proceed, and updates retryAfter
// with the number of seconds the caller should wait if denied.
func (tb *tokenBucket) allow() (ok bool, retryAfter int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tb.lastFill = now

	// Refill tokens that have accrued since last check.
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.maxBurst {
		tb.tokens = tb.maxBurst
	}

	if tb.tokens >= 1 {
		tb.tokens--
		return true, 0
	}

	// Tell the caller how many seconds until a token is available.
	wait := (1 - tb.tokens) / tb.rate
	return false, int(wait) + 1
}

// rateLimitMiddleware wraps an http.Handler, returning 429 Too Many Requests
// when the token bucket is empty. The Retry-After header is set on 429 responses.
func rateLimitMiddleware(next http.Handler, limiter *tokenBucket) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, retryAfter := limiter.allow()
		if !ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			http.Error(w, "rate limit exceeded — try again later", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
