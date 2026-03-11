package engine

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Reeteshrajesh/runway/internal/manifest"
)

const (
	defaultHealthInterval = 2  // seconds between polls
	defaultHealthRetries  = 10 // max attempts
)

// waitHealthy polls cfg.URL until it returns HTTP 200 or retries are exhausted.
// Returns nil on success, an error after all retries fail.
// Returns nil immediately if cfg.URL is empty (health check not configured).
func waitHealthy(cfg manifest.HealthCheck, logf func(string, ...any)) error {
	if cfg.URL == "" {
		return nil
	}

	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultHealthInterval
	}
	retries := cfg.Retries
	if retries <= 0 {
		retries = defaultHealthRetries
	}

	client := &http.Client{Timeout: 5 * time.Second}

	logf("health check: polling %s (interval=%ds, retries=%d)", cfg.URL, interval, retries)

	for attempt := 1; attempt <= retries; attempt++ {
		resp, err := client.Get(cfg.URL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				logf("health check: OK (attempt %d/%d)", attempt, retries)
				return nil
			}
			logf("health check: attempt %d/%d — HTTP %d", attempt, retries, resp.StatusCode)
		} else {
			logf("health check: attempt %d/%d — %v", attempt, retries, err)
		}

		if attempt < retries {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}

	return fmt.Errorf("health check failed after %d attempts: %s did not return HTTP 200", retries, cfg.URL)
}
