// Package webhook provides an HTTP server that receives Git push webhooks,
// verifies their HMAC-SHA256 signatures, and triggers deployments.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"os"
	"path/filepath"

	"github.com/Reeteshrajesh/runway/internal/engine"
	"github.com/Reeteshrajesh/runway/internal/logger"
	"github.com/Reeteshrajesh/runway/internal/multiapp"
)

const (
	// maxBodyBytes limits webhook payload size to prevent memory exhaustion.
	maxBodyBytes = 5 * 1024 * 1024 // 5MB

	signatureHeader = "X-Hub-Signature-256"
	eventHeader     = "X-GitHub-Event"
)

// Config holds the webhook server configuration.
type Config struct {
	// Port is the TCP port to listen on.
	Port int

	// Secret is the HMAC signing secret configured in GitHub/GitLab.
	Secret string

	// DeployConfig is the base engine config (Commit will be filled per-request).
	DeployConfig engine.Config

	// EventLog emits structured deploy lifecycle events (text or JSON).
	// If nil, a default text logger writing to stderr is used.
	EventLog *logger.EventLogger

	// RateLimit is the maximum number of webhook requests allowed per minute.
	// Requests exceeding this limit receive HTTP 429 with a Retry-After header.
	// Zero or negative values disable rate limiting (no limit).
	RateLimit int
}

// Server is a lightweight HTTP server for receiving Git webhooks.
type Server struct {
	cfg      Config
	mux      *http.ServeMux
	server   *http.Server
	deploys  sync.WaitGroup // tracks in-flight deploy goroutines
	eventLog *logger.EventLogger
	limiter  *tokenBucket // nil when rate limiting is disabled
}

// New creates a new webhook Server with the given configuration.
func New(cfg Config) *Server {
	el := cfg.EventLog
	if el == nil {
		el = logger.DefaultEventLogger()
	}

	var limiter *tokenBucket
	if cfg.RateLimit > 0 {
		limiter = newTokenBucket(cfg.RateLimit)
	}

	s := &Server{cfg: cfg, eventLog: el, limiter: limiter}

	s.mux = http.NewServeMux()

	// The /webhook handler is wrapped with rate limiting when configured.
	var webhookHandler http.Handler = http.HandlerFunc(s.handleWebhook)
	if limiter != nil {
		webhookHandler = rateLimitMiddleware(webhookHandler, limiter)
	}
	s.mux.Handle("/webhook", webhookHandler)
	s.mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Handler returns the underlying http.Handler for use in tests.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start begins listening for incoming webhook requests.
// This is a blocking call; run it in a goroutine if needed.
func (s *Server) Start() error {
	fmt.Printf("runway listening on :%d\n", s.cfg.Port)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP listener and waits for any in-flight
// deploy goroutine to complete before returning. The context controls how long
// to wait for the HTTP server itself to drain open connections; deploy
// goroutines are always awaited (they carry their own deploy timeout).
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	// Always wait for in-flight deploys regardless of HTTP drain errors.
	s.deploys.Wait()
	return err
}

// handleWebhook processes incoming GitHub push webhook requests.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit body size before reading to prevent memory exhaustion.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// ── HMAC signature verification ───────────────────────────────────────────
	sig := r.Header.Get(signatureHeader)
	if !verifySignature(body, s.cfg.Secret, sig) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// ── Only process push events ──────────────────────────────────────────────
	event := r.Header.Get(eventHeader)
	if event != "push" && event != "" {
		// Accept unknown event headers (e.g. GitLab doesn't send X-GitHub-Event)
		// but skip non-push GitHub events.
		if event != "" && event != "push" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "event %q ignored\n", event)
			return
		}
	}

	// ── Parse payload to extract commit SHA and branch ──────────────────────
	commit, err := extractCommit(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("payload parse error: %v", err), http.StatusBadRequest)
		return
	}

	if commit == "" {
		http.Error(w, "could not determine commit SHA from payload", http.StatusBadRequest)
		return
	}

	branch := extractBranch(body)

	// ── Multi-app fan-out or single-app deploy ───────────────────────────────
	// If runway.yml exists in the base directory, fan out to all matching apps.
	// Otherwise fall back to the single-app config.
	baseDir := s.cfg.DeployConfig.BaseDir
	if baseDir == "" {
		baseDir = "/opt/runway"
	}
	multiCfg, multiErr := multiapp.ParseFile(filepath.Join(baseDir, "runway.yml"))
	if multiErr == nil && len(multiCfg.Apps) > 0 {
		// Multi-app mode: trigger a deploy for each app that allows this branch.
		for _, app := range multiCfg.Apps {
			if !app.BranchAllowed(branch) {
				fmt.Fprintf(os.Stderr, "[runway] multi-app: skipping %s (branch %q not allowed)\n", app.Name, branch)
				continue
			}
			appCfg := engine.Config{
				BaseDir:   app.BaseDir,
				RepoURL:   app.Repo,
				Commit:    commit,
				Branch:    branch,
				GitToken:  os.Getenv("GITOPS_GIT_TOKEN"),
				Triggered: "webhook",
			}
			s.deploys.Add(1)
			go func(cfg engine.Config, name string) {
				defer s.deploys.Done()
				s.eventLog.DeployStart(commit, "webhook")
				result := engine.Deploy(cfg)
				dur := result.EndedAt.Sub(result.StartedAt).Seconds()
				if result.Err != nil {
					if result.AutoRolledBack {
						s.eventLog.DeployRolledBack(commit, result.RolledBackTo, "webhook", dur)
					} else {
						s.eventLog.DeployFailed(commit, "webhook", dur, result.Err)
					}
				} else {
					s.eventLog.DeploySuccess(commit, "webhook", dur)
				}
			}(appCfg, app.Name)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprintf(w, "deployment queued for commit %s (%d apps)\n", commit, len(multiCfg.Apps))
		return
	}

	// Single-app mode (no runway.yml or parse error).
	// ── Trigger deployment (non-blocking) ─────────────────────────────────────
	cfg := s.cfg.DeployConfig
	cfg.Commit = commit
	cfg.Branch = branch
	cfg.Triggered = "webhook"

	s.deploys.Add(1)
	go func() {
		defer s.deploys.Done()
		s.eventLog.DeployStart(commit, "webhook")
		result := engine.Deploy(cfg)
		dur := result.EndedAt.Sub(result.StartedAt).Seconds()
		if result.Err != nil {
			if result.AutoRolledBack {
				s.eventLog.DeployRolledBack(commit, result.RolledBackTo, "webhook", dur)
			} else {
				s.eventLog.DeployFailed(commit, "webhook", dur, result.Err)
			}
		} else {
			s.eventLog.DeploySuccess(commit, "webhook", dur)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprintf(w, "deployment queued for commit %s\n", commit)
}

// handleHealth returns 200 OK for simple uptime checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "ok")
}

// verifySignature validates an X-Hub-Signature-256 header against the body and secret.
// Uses hmac.Equal (constant-time) to prevent timing attacks.
func verifySignature(body []byte, secret, header string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}

	got, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)

	// constant-time comparison — mandatory to prevent timing side-channel attacks.
	return hmac.Equal(got, expected)
}

// pushPayload represents the minimal fields we need from a GitHub push event.
type pushPayload struct {
	Ref        string `json:"ref"`   // e.g. "refs/heads/main"
	After      string `json:"after"` // commit SHA after the push
	HeadCommit *struct {
		ID string `json:"id"`
	} `json:"head_commit"`
}

// extractCommit parses the webhook payload and returns the pushed commit SHA.
// Supports GitHub push events. Falls back to "after" field if head_commit is absent.
func extractCommit(body []byte) (string, error) {
	var payload pushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}

	if payload.HeadCommit != nil && payload.HeadCommit.ID != "" {
		return payload.HeadCommit.ID, nil
	}

	if payload.After != "" && payload.After != "0000000000000000000000000000000000000000" {
		return payload.After, nil
	}

	return "", nil
}

// extractBranch parses the webhook payload and returns the short branch name.
// e.g. "refs/heads/main" → "main". Returns "" if the ref is not a branch.
func extractBranch(body []byte) string {
	var payload pushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	const prefix = "refs/heads/"
	if strings.HasPrefix(payload.Ref, prefix) {
		return strings.TrimPrefix(payload.Ref, prefix)
	}
	return ""
}
