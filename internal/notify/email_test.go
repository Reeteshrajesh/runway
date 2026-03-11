package notify_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Reeteshrajesh/runway/internal/manifest"
	"github.com/Reeteshrajesh/runway/internal/notify"
)

// TestSendDeployEmail_NoOp verifies that sending with an empty config is a
// silent no-op (returns nil without attempting any SMTP connection).
func TestSendDeployEmail_NoOp(t *testing.T) {
	err := notify.SendDeployEmail(manifest.NotifyEmail{}, notify.DeployEvent{
		App:    "myapp",
		Commit: "abc123",
	})
	if err != nil {
		t.Errorf("expected nil for unconfigured notify, got: %v", err)
	}
}

func TestSendDeployEmail_NoOpMissingHost(t *testing.T) {
	err := notify.SendDeployEmail(manifest.NotifyEmail{To: "user@example.com"}, notify.DeployEvent{})
	if err != nil {
		t.Errorf("expected nil when smtp_host is missing, got: %v", err)
	}
}

// TestComposeEmail_* tests are done indirectly via the exported BuildMessageForTest
// helper exposed in test helpers below. Since composeEmail is unexported, we
// test it through the observable email content by pointing at a local SMTP stub.
// For unit coverage, we verify the subject/body logic via a package-level
// test helper that wraps composeEmail.

func TestDeployEvent_SuccessSubject(t *testing.T) {
	ev := notify.DeployEvent{
		App:    "myapp",
		Commit: "abc123def456789",
		Err:    nil,
	}
	subj, body := notify.ComposeEmailForTest(manifest.NotifyEmail{}, ev)
	if !strings.Contains(subj, "deployed") {
		t.Errorf("success subject = %q, want to contain 'deployed'", subj)
	}
	if !strings.Contains(body, "myapp") {
		t.Errorf("body missing app name: %q", body)
	}
}

func TestDeployEvent_FailureSubject(t *testing.T) {
	ev := notify.DeployEvent{
		App:    "myapp",
		Commit: "abc123",
		Err:    errors.New("build failed"),
	}
	subj, body := notify.ComposeEmailForTest(manifest.NotifyEmail{}, ev)
	if !strings.Contains(subj, "failed") {
		t.Errorf("failure subject = %q, want to contain 'failed'", subj)
	}
	if !strings.Contains(body, "build failed") {
		t.Errorf("body missing error: %q", body)
	}
}

func TestDeployEvent_RolledBackSubject(t *testing.T) {
	ev := notify.DeployEvent{
		App:          "myapp",
		Commit:       "bad123",
		Err:          errors.New("start failed"),
		RolledBack:   true,
		RolledBackTo: "good456",
		Duration:     42 * time.Second,
	}
	subj, _ := notify.ComposeEmailForTest(manifest.NotifyEmail{}, ev)
	if !strings.Contains(subj, "rolled") {
		t.Errorf("rollback subject = %q, want to contain 'rolled'", subj)
	}
}

func TestDeployEvent_LogLinesIncludedOnFailure(t *testing.T) {
	ev := notify.DeployEvent{
		App:          "myapp",
		Commit:       "abc",
		Err:          errors.New("oops"),
		LastLogLines: []string{"line1", "line2", "line3"},
	}
	_, body := notify.ComposeEmailForTest(manifest.NotifyEmail{}, ev)
	if !strings.Contains(body, "line2") {
		t.Errorf("body missing log lines: %q", body)
	}
}
