package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/cli"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		}
	})
}

// ── Run() dispatch ────────────────────────────────────────────────────────────

func TestRun_NoArgs_ReturnsNil(t *testing.T) {
	// runway with no args prints usage and returns nil.
	err := cli.Run([]string{}, "test")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestRun_Version(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			err := cli.Run([]string{arg}, "1.2.3")
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
		})
	}
}

func TestRun_Help(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			err := cli.Run([]string{arg}, "test")
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
		})
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := cli.Run([]string{"foobar"}, "test")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error should mention unknown command, got: %v", err)
	}
}

func TestRun_NoColorFlag_Stripped(t *testing.T) {
	// --no-color is a global flag; it should not be passed to sub-commands.
	// Passing it before an unknown command should still return "unknown command"
	// rather than a flag-parse error.
	err := cli.Run([]string{"--no-color", "foobar"}, "test")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected unknown-command error, got: %v", err)
	}
}

// ── ExitCode ──────────────────────────────────────────────────────────────────

func TestExitCode_Nil(t *testing.T) {
	// ExitCode on nil shouldn't be called in practice, but the function
	// should not panic.
	// Actually ExitCode expects a non-nil error by contract; just test typed errors.
}

func TestExitCode_ExitError(t *testing.T) {
	tests := []struct {
		code int
	}{
		{cli.ExitOK},
		{cli.ExitGeneralError},
		{cli.ExitLockHeld},
		{cli.ExitBuildFailed},
		{cli.ExitStartFailed},
		{cli.ExitGitError},
		{cli.ExitManifestError},
		{cli.ExitNotFound},
	}
	for _, tt := range tests {
		err := &cli.ExitError{Code: tt.code, Err: errors.New("test")}
		got := cli.ExitCode(err)
		if got != tt.code {
			t.Errorf("ExitCode(%d): got %d", tt.code, got)
		}
	}
}

func TestExitCode_PlainError_ReturnsGeneralError(t *testing.T) {
	err := errors.New("some error")
	got := cli.ExitCode(err)
	if got != cli.ExitGeneralError {
		t.Errorf("expected ExitGeneralError(%d), got %d", cli.ExitGeneralError, got)
	}
}

func TestExitError_Error(t *testing.T) {
	inner := errors.New("something failed")
	e := &cli.ExitError{Code: cli.ExitBuildFailed, Err: inner}
	if e.Error() != inner.Error() {
		t.Errorf("ExitError.Error() = %q, want %q", e.Error(), inner.Error())
	}
}

func TestExitError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	e := &cli.ExitError{Code: cli.ExitGitError, Err: inner}
	if !errors.Is(e, inner) {
		t.Error("errors.Is should find the wrapped inner error via Unwrap")
	}
}

// ── deploy command ────────────────────────────────────────────────────────────

func TestRunDeploy_MissingCommit(t *testing.T) {
	setEnv(t, "GITOPS_REPO", "git@github.com:org/repo.git")
	err := cli.Run([]string{"deploy"}, "test")
	if err == nil {
		t.Fatal("expected error when no commit given")
	}
}

func TestRunDeploy_MissingRepo(t *testing.T) {
	unsetEnv(t, "GITOPS_REPO")
	err := cli.Run([]string{"deploy", "abc123"}, "test")
	if err == nil {
		t.Fatal("expected error when GITOPS_REPO is not set")
	}
	if cli.ExitCode(err) != cli.ExitManifestError {
		t.Errorf("expected ExitManifestError, got code %d: %v", cli.ExitCode(err), err)
	}
}

func TestRunDeploy_DryRun_MissingRepo(t *testing.T) {
	unsetEnv(t, "GITOPS_REPO")
	err := cli.Run([]string{"deploy", "--dry-run", "abc123"}, "test")
	if err == nil {
		t.Fatal("expected error when GITOPS_REPO is not set")
	}
}

// ── rollback command ──────────────────────────────────────────────────────────

func TestRunRollback_MissingCommit(t *testing.T) {
	err := cli.Run([]string{"rollback"}, "test")
	if err == nil {
		t.Fatal("expected error when no commit given")
	}
}

// ── log command ───────────────────────────────────────────────────────────────

func TestRunLog_MissingCommit(t *testing.T) {
	err := cli.Run([]string{"log"}, "test")
	if err == nil {
		t.Fatal("expected error when no commit given")
	}
}

func TestRunLog_NotFound(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "GITOPS_DIR", dir)
	err := cli.Run([]string{"log", "deadbeef"}, "test")
	if err == nil {
		t.Fatal("expected error for non-existent log")
	}
}

func TestRunLog_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "GITOPS_DIR", dir)

	commit := "abc123def456"
	logDir := filepath.Join(dir, "releases", commit)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	logContent := "=== deploy log ===\nall good\n"
	if err := os.WriteFile(filepath.Join(logDir, "deploy.log"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Redirect stdout to capture output.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cli.Run([]string{"log", commit}, "test")

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(buf.String(), "all good") {
		t.Errorf("expected log content in output, got: %q", buf.String())
	}
}

// ── history command ───────────────────────────────────────────────────────────

func TestRunHistory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "GITOPS_DIR", dir)

	// No history.json yet — should print "(no deployments yet)" and return nil.
	err := cli.Run([]string{"history"}, "test")
	if err != nil {
		t.Errorf("expected nil error for empty history, got %v", err)
	}
}

func TestRunHistory_InvalidLimit(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "GITOPS_DIR", dir)
	// Invalid flag value.
	err := cli.Run([]string{"history", "--limit", "notanumber"}, "test")
	if err == nil {
		t.Fatal("expected error for invalid --limit value")
	}
}

// ── doctor command ────────────────────────────────────────────────────────────

func TestRunDoctor_MissingDir(t *testing.T) {
	setEnv(t, "GITOPS_DIR", "/tmp/runway-does-not-exist-xyz")
	unsetEnv(t, "GITOPS_REPO")

	err := cli.Run([]string{"doctor"}, "test")
	// doctor reports issues via exit code 1, not nil
	if err == nil {
		t.Fatal("expected error when GITOPS_DIR and GITOPS_REPO are missing")
	}
}

// ── init command ──────────────────────────────────────────────────────────────

func TestRunInit_InvalidFlag(t *testing.T) {
	err := cli.Run([]string{"init", "--unknown-flag"}, "test")
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

// ── dry-run (runDryRun) integration ───────────────────────────────────────────

func TestRunDeploy_DryRun_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "GITOPS_DIR", dir)
	setEnv(t, "GITOPS_REPO", "git@github.com:org/repo.git")
	// releases/ exists but no manifest.yml
	os.MkdirAll(filepath.Join(dir, "releases"), 0755)

	err := cli.Run([]string{"deploy", "--dry-run", "abc123"}, "test")
	// dry-run should return an error because manifest is missing
	if err == nil {
		t.Fatal("expected error when manifest.yml is absent")
	}
}

func TestRunDeploy_DryRun_ValidSetup(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "GITOPS_DIR", dir)
	setEnv(t, "GITOPS_REPO", "git@github.com:org/repo.git")

	os.MkdirAll(filepath.Join(dir, "releases"), 0755)

	manifest := "app: test-app\nstart:\n  - ./start.sh\n"
	os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(manifest), 0644)

	err := cli.Run([]string{"deploy", "--dry-run", "abc123def456"}, "test")
	if err != nil {
		t.Errorf("expected nil error for valid dry-run setup, got: %v", err)
	}
}

// ── listen command ────────────────────────────────────────────────────────────

func TestRunListen_MissingSecret(t *testing.T) {
	unsetEnv(t, "GITOPS_WEBHOOK_SECRET")
	setEnv(t, "GITOPS_REPO", "git@github.com:org/repo.git")
	err := cli.Run([]string{"listen", "--port", "19999"}, "test")
	if err == nil {
		t.Fatal("expected error when --secret is not provided")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("error should mention secret, got: %v", err)
	}
}

func TestRunListen_MissingRepo(t *testing.T) {
	unsetEnv(t, "GITOPS_REPO")
	err := cli.Run([]string{"listen", "--port", "19999", "--secret", "s3cr3t"}, "test")
	if err == nil {
		t.Fatal("expected error when GITOPS_REPO is not set")
	}
	if !strings.Contains(err.Error(), "GITOPS_REPO") {
		t.Errorf("error should mention GITOPS_REPO, got: %v", err)
	}
}
