package webhook_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/webhook"
)

const testSecret = "super-secret-webhook-key"

// sign computes the GitHub-style HMAC-SHA256 signature for a payload.
func sign(t *testing.T, body []byte, secret string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// pushPayload builds a minimal GitHub push event body for the given commit.
func pushPayload(commit string) []byte {
	payload := map[string]any{
		"after": commit,
		"head_commit": map[string]string{
			"id": commit,
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

// newTestServer returns an httptest.Server backed by a webhook.Server.
// The DeployConfig is intentionally left empty — we only test HTTP handling,
// not the actual deployment logic.
func newTestServer(t *testing.T, secret string) *httptest.Server {
	t.Helper()
	cfg := webhook.Config{
		Port:   0, // not used; httptest handles the listener
		Secret: secret,
	}
	srv := webhook.New(cfg)
	return httptest.NewServer(srv.Handler())
}

func TestWebhook_ValidSignature_Accepted(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	body := pushPayload("abc123def456")
	sig := sign(t, body, testSecret)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestWebhook_InvalidSignature_Rejected(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	body := pushPayload("abc123")

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
	req.Header.Set("X-GitHub-Event", "push")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (invalid sig should be rejected)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestWebhook_MissingSignature_Rejected(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	body := pushPayload("abc123")

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Hub-Signature-256 header.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (missing sig should be rejected)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestWebhook_WrongSecret_Rejected(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	body := pushPayload("abc123")
	// Sign with a different secret.
	sig := sign(t, body, "wrong-secret")

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestWebhook_GetMethod_Rejected(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/webhook")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestWebhook_NonPushEvent_Ignored(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	body := []byte(`{}`)
	sig := sign(t, body, testSecret)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "pull_request") // not a push

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d (non-push event should be ignored, not rejected)", resp.StatusCode, http.StatusOK)
	}
}

func TestWebhook_HealthEndpoint(t *testing.T) {
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestWebhook_SignatureTimingConsistency(t *testing.T) {
	// Verify that both a valid and invalid signature take the same code path
	// (both go through hmac.Equal, not an early return on length mismatch alone).
	// This is a structural test — we confirm valid sig succeeds and tampered fails.
	ts := newTestServer(t, testSecret)
	defer ts.Close()

	body := pushPayload("deadbeef")
	validSig := sign(t, body, testSecret)

	// Tamper with a single byte of the hex signature.
	sigBytes := []byte(validSig)
	sigBytes[len(sigBytes)-1] ^= 0x01
	tamperedSig := string(sigBytes)

	for _, tc := range []struct {
		sig        string
		wantStatus int
	}{
		{validSig, http.StatusAccepted},
		{tamperedSig, http.StatusUnauthorized},
		{fmt.Sprintf("sha256=%s", hex.EncodeToString(make([]byte, 32))), http.StatusUnauthorized},
	} {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", tc.sig)
		req.Header.Set("X-GitHub-Event", "push")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != tc.wantStatus {
			t.Errorf("sig=%q: status = %d, want %d", tc.sig[:20], resp.StatusCode, tc.wantStatus)
		}
	}
}
