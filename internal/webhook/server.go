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
	"time"

	"github.com/Reeteshrajesh/runway/internal/engine"
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
}

// Server is a lightweight HTTP server for receiving Git webhooks.
type Server struct {
	cfg    Config
	mux    *http.ServeMux
	server *http.Server
}

// New creates a new webhook Server with the given configuration.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg}

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/webhook", s.handleWebhook)
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

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
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

	// ── Parse payload to extract commit SHA ───────────────────────────────────
	commit, err := extractCommit(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("payload parse error: %v", err), http.StatusBadRequest)
		return
	}

	if commit == "" {
		http.Error(w, "could not determine commit SHA from payload", http.StatusBadRequest)
		return
	}

	// ── Trigger deployment (non-blocking) ─────────────────────────────────────
	// Run in a goroutine so the webhook handler returns immediately (GitHub
	// expects a response within 10 seconds or it retries).
	cfg := s.cfg.DeployConfig
	cfg.Commit = commit
	cfg.Triggered = "webhook"

	go func() {
		result := engine.Deploy(cfg)
		if result.Err != nil {
			fmt.Printf("deploy failed [%s]: %v\n", commit, result.Err)
		} else {
			fmt.Printf("deploy succeeded [%s]\n", commit)
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
	After  string `json:"after"`  // commit SHA after the push
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
