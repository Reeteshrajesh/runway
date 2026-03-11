package logger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/logger"
)

func TestNew_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	l, err := logger.New(dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	l.Logf("hello %s", "world")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "deploy.log"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("log file missing expected content, got: %s", data)
	}
}

func TestNew_TeeWriter(t *testing.T) {
	dir := t.TempDir()
	var tee bytes.Buffer
	l, err := logger.New(dir, &tee)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	l.Logf("tee test")
	_ = l.Close()

	if !strings.Contains(tee.String(), "tee test") {
		t.Errorf("tee missing content, got: %q", tee.String())
	}
}

func TestNew_WriteRawBytes(t *testing.T) {
	dir := t.TempDir()
	var tee bytes.Buffer
	l, err := logger.New(dir, &tee)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, _ = l.Write([]byte("raw output line\n"))
	_ = l.Close()

	if !strings.Contains(tee.String(), "raw output line") {
		t.Errorf("tee missing raw output, got: %q", tee.String())
	}
}

func TestNewStreaming_PrefixesLines(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	l, err := logger.NewStreaming(dir, "[runway] ", &out)
	if err != nil {
		t.Fatalf("NewStreaming: %v", err)
	}

	_, _ = l.Write([]byte("building project\n"))
	_, _ = l.Write([]byte("done\n"))
	_ = l.Close()

	got := out.String()
	if !strings.Contains(got, "[runway] building project") {
		t.Errorf("streaming output missing prefix, got: %q", got)
	}
	if !strings.Contains(got, "[runway] done") {
		t.Errorf("streaming output missing second line, got: %q", got)
	}
}

func TestNewStreaming_FileHasNoPrefix(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	l, err := logger.NewStreaming(dir, "[runway] ", &out)
	if err != nil {
		t.Fatalf("NewStreaming: %v", err)
	}

	_, _ = l.Write([]byte("clean log line\n"))
	_ = l.Close()

	data, _ := os.ReadFile(filepath.Join(dir, "deploy.log"))
	if strings.Contains(string(data), "[runway]") {
		t.Errorf("log file should NOT contain prefix, got: %s", data)
	}
	if !strings.Contains(string(data), "clean log line") {
		t.Errorf("log file missing raw content, got: %s", data)
	}
}

func TestStep_WritesToTee(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	l, err := logger.New(dir, &out)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	l.Step("→ running build")
	_ = l.Close()

	if !strings.Contains(out.String(), "→ running build") {
		t.Errorf("Step output missing from tee, got: %q", out.String())
	}

	// Step must NOT appear in the log file.
	data, _ := os.ReadFile(filepath.Join(dir, "deploy.log"))
	if strings.Contains(string(data), "→ running build") {
		t.Errorf("Step should not write to log file, got: %s", data)
	}
}

func TestPath_ReturnsLogFilePath(t *testing.T) {
	dir := t.TempDir()
	l, err := logger.New(dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = l.Close()

	if !strings.HasSuffix(l.Path(), "deploy.log") {
		t.Errorf("Path() = %q, want suffix 'deploy.log'", l.Path())
	}
}
