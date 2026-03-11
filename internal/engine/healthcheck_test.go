package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/manifest"
)

func noopLog(format string, args ...any) {}

func TestWaitHealthy_NoURL_ReturnsNil(t *testing.T) {
	cfg := manifest.HealthCheck{URL: ""}
	if err := waitHealthy(cfg, noopLog); err != nil {
		t.Errorf("expected nil for empty URL, got %v", err)
	}
}

func TestWaitHealthy_Immediately200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := manifest.HealthCheck{URL: srv.URL, Retries: 3, Interval: 0}
	if err := waitHealthy(cfg, noopLog); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWaitHealthy_NonOKStatus_Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := manifest.HealthCheck{URL: srv.URL, Retries: 2, Interval: 1}
	err := waitHealthy(cfg, noopLog)
	if err == nil {
		t.Fatal("expected error when server always returns 503")
	}
}

func TestWaitHealthy_EventuallyHealthy(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := manifest.HealthCheck{URL: srv.URL, Retries: 5, Interval: 1}
	if err := waitHealthy(cfg, noopLog); err != nil {
		t.Errorf("expected nil after eventual health, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 fail + 1 success), got %d", calls)
	}
}

func TestWaitHealthy_DefaultsApplied(t *testing.T) {
	// Zero values for Interval and Retries should use defaults without panic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := manifest.HealthCheck{URL: srv.URL} // zero Interval and Retries
	if err := waitHealthy(cfg, noopLog); err != nil {
		t.Errorf("expected nil with defaults, got %v", err)
	}
}

func TestWaitHealthy_ConnectionRefused_Fails(t *testing.T) {
	cfg := manifest.HealthCheck{
		URL:      "http://127.0.0.1:19999", // nothing listening
		Retries:  2,
		Interval: 0,
	}
	err := waitHealthy(cfg, noopLog)
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}
