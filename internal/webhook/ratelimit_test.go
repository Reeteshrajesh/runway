package webhook

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenBucket_AllowsUpToLimit(t *testing.T) {
	tb := newTokenBucket(5) // 5 per minute

	for i := 0; i < 5; i++ {
		ok, _ := tb.allow()
		if !ok {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
}

func TestTokenBucket_DeniesOverLimit(t *testing.T) {
	tb := newTokenBucket(3)

	// Drain the bucket.
	for i := 0; i < 3; i++ {
		tb.allow()
	}

	ok, retryAfter := tb.allow()
	if ok {
		t.Fatal("4th request should have been denied")
	}
	if retryAfter <= 0 {
		t.Errorf("retryAfter should be positive, got %d", retryAfter)
	}
}

func TestRateLimitMiddleware_Returns429(t *testing.T) {
	tb := newTokenBucket(2)
	// Drain the bucket.
	tb.allow()
	tb.allow()

	handler := rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), tb)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set")
	}
}

func TestRateLimitMiddleware_PassesWhenTokensAvailable(t *testing.T) {
	tb := newTokenBucket(10)

	handler := rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), tb)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}
}

func TestNewServer_RateLimitDisabledWhenZero(t *testing.T) {
	cfg := Config{
		Port:      9099,
		Secret:    "test",
		RateLimit: 0, // disabled
	}
	s := New(cfg)
	if s.limiter != nil {
		t.Error("limiter should be nil when RateLimit=0")
	}
}

func TestNewServer_RateLimitEnabledWhenPositive(t *testing.T) {
	cfg := Config{
		Port:      9099,
		Secret:    "test",
		RateLimit: 5,
	}
	s := New(cfg)
	if s.limiter == nil {
		t.Error("limiter should be non-nil when RateLimit>0")
	}
}
