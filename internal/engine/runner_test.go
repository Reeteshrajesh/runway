package engine_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Reeteshrajesh/runway/internal/engine"
)

func TestRunCommand_Success(t *testing.T) {
	var buf bytes.Buffer
	err := engine.RunCommand(context.Background(), "echo hello", engine.RunOptions{
		Stdout: &buf,
		Stderr: &buf,
	})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("output = %q, want to contain 'hello'", buf.String())
	}
}

func TestRunCommand_Failure(t *testing.T) {
	err := engine.RunCommand(context.Background(), "exit 1", engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error for failing command, got nil")
	}
}

func TestRunCommand_NonExistentBinary(t *testing.T) {
	err := engine.RunCommand(context.Background(), "this_binary_does_not_exist_xyz", engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error for non-existent binary, got nil")
	}
}

func TestRunCommand_CapturesStdout(t *testing.T) {
	var out bytes.Buffer
	_ = engine.RunCommand(context.Background(), "printf 'line1\nline2'", engine.RunOptions{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
	})

	output := out.String()
	if !strings.Contains(output, "line1") || !strings.Contains(output, "line2") {
		t.Errorf("output = %q, expected both lines", output)
	}
}

func TestRunCommand_CapturesStderr(t *testing.T) {
	var stderr bytes.Buffer
	_ = engine.RunCommand(context.Background(), "echo err >&2", engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	})

	if !strings.Contains(stderr.String(), "err") {
		t.Errorf("stderr = %q, expected 'err'", stderr.String())
	}
}

func TestRunCommand_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	err := engine.RunCommand(context.Background(), "pwd", engine.RunOptions{
		Dir:    dir,
		Stdout: &out,
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("RunCommand pwd: %v", err)
	}

	// Output should contain the temp dir path.
	if !strings.Contains(out.String(), dir) {
		t.Errorf("pwd output = %q, want to contain %q", out.String(), dir)
	}
}

func TestRunCommand_EnvInjection(t *testing.T) {
	var out bytes.Buffer

	err := engine.RunCommand(context.Background(), "echo $MY_TEST_VAR", engine.RunOptions{
		Env:    []string{"MY_TEST_VAR=injected_value"},
		Stdout: &out,
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}

	if !strings.Contains(out.String(), "injected_value") {
		t.Errorf("output = %q, expected 'injected_value'", out.String())
	}
}

// TestRunCommand_ContextCancellation verifies that a running command is killed
// when its context is cancelled before the command completes.
func TestRunCommand_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately before the command can finish.
	cancel()

	err := engine.RunCommand(ctx, "sleep 10", engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
	// The error must wrap context.Canceled.
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, expected to wrap context.Canceled", err)
	}
}

// TestRunCommand_ContextTimeout verifies that a long-running command is killed
// when the deadline expires.
func TestRunCommand_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := engine.RunCommand(ctx, "sleep 10", engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error when context times out, got nil")
	}
	// The error must wrap context.DeadlineExceeded.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, expected to wrap context.DeadlineExceeded", err)
	}
}

func TestRunCommands_AllSucceed(t *testing.T) {
	var out bytes.Buffer
	cmds := []string{"echo one", "echo two", "echo three"}

	err := engine.RunCommands(context.Background(), cmds, engine.RunOptions{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("RunCommands: %v", err)
	}

	output := out.String()
	for _, word := range []string{"one", "two", "three"} {
		if !strings.Contains(output, word) {
			t.Errorf("output = %q, missing %q", output, word)
		}
	}
}

func TestRunCommands_StopsOnFirstFailure(t *testing.T) {
	var out bytes.Buffer
	cmds := []string{"echo first", "exit 1", "echo third"}

	err := engine.RunCommands(context.Background(), cmds, engine.RunOptions{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error when a command fails, got nil")
	}

	// "third" should NOT appear — execution must have stopped at step 2.
	if strings.Contains(out.String(), "third") {
		t.Errorf("output contains 'third', but execution should have stopped at step 2")
	}
}

func TestRunCommands_ErrorMessageContainsStepInfo(t *testing.T) {
	cmds := []string{"echo ok", "exit 42", "echo never"}

	err := engine.RunCommands(context.Background(), cmds, engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Error message should mention the step number.
	if !strings.Contains(err.Error(), "step 2") {
		t.Errorf("error = %q, expected step number in message", err.Error())
	}
}

func TestRunCommands_EmptyList(t *testing.T) {
	err := engine.RunCommands(context.Background(), nil, engine.RunOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Errorf("empty command list should succeed, got: %v", err)
	}
}

// TestRunCommands_ContextCancelledMidway verifies that RunCommands stops
// executing further steps when its context is cancelled between steps.
func TestRunCommands_ContextCancelledMidway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var out bytes.Buffer
	// The second command cancels the context; the third must never run.
	cmds := []string{
		"echo step1",
		"sleep 10",
		"echo step3",
	}

	// Cancel after a short delay so step1 completes but step2 gets killed.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := engine.RunCommands(ctx, cmds, engine.RunOptions{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error when context is cancelled midway, got nil")
	}
	if strings.Contains(out.String(), "step3") {
		t.Error("step3 should not have run after context cancellation")
	}
}
