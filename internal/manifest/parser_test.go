package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/manifest"
)

// writeManifest creates a temporary manifest file with the given content
// and returns its path. The file is cleaned up automatically by t.Cleanup.
func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	return path
}

func TestParseFile_ValidFull(t *testing.T) {
	path := writeManifest(t, `
app: my-service
env_file: .env

setup:
  - npm install
  - npm ci

build:
  - npm run build

start:
  - pm2 restart app || pm2 start dist/index.js
`)

	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.App != "my-service" {
		t.Errorf("App = %q, want %q", m.App, "my-service")
	}
	if m.EnvFile != ".env" {
		t.Errorf("EnvFile = %q, want %q", m.EnvFile, ".env")
	}
	if len(m.Setup) != 2 {
		t.Errorf("Setup len = %d, want 2", len(m.Setup))
	}
	if m.Setup[0] != "npm install" {
		t.Errorf("Setup[0] = %q, want %q", m.Setup[0], "npm install")
	}
	if len(m.Build) != 1 || m.Build[0] != "npm run build" {
		t.Errorf("Build = %v, want [npm run build]", m.Build)
	}
	if len(m.Start) != 1 {
		t.Errorf("Start len = %d, want 1", len(m.Start))
	}
}

func TestParseFile_MinimalValid(t *testing.T) {
	path := writeManifest(t, `
app: minimal
start:
  - ./run.sh
`)

	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.App != "minimal" {
		t.Errorf("App = %q, want %q", m.App, "minimal")
	}
	if len(m.Setup) != 0 {
		t.Errorf("Setup = %v, want empty", m.Setup)
	}
	if len(m.Build) != 0 {
		t.Errorf("Build = %v, want empty", m.Build)
	}
	if m.Start[0] != "./run.sh" {
		t.Errorf("Start[0] = %q, want %q", m.Start[0], "./run.sh")
	}
}

func TestParseFile_CommentsAndBlankLines(t *testing.T) {
	path := writeManifest(t, `
# this is a comment
app: svc

# another comment

start:
  # inline comment should not appear as a command
  - ./start.sh
`)

	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.App != "svc" {
		t.Errorf("App = %q, want %q", m.App, "svc")
	}
	if len(m.Start) != 1 || m.Start[0] != "./start.sh" {
		t.Errorf("Start = %v, want [./start.sh]", m.Start)
	}
}

func TestParseFile_QuotedValues(t *testing.T) {
	path := writeManifest(t, `
app: "quoted-app"
start:
  - 'single quoted command'
`)

	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.App != "quoted-app" {
		t.Errorf("App = %q, want %q", m.App, "quoted-app")
	}
	if m.Start[0] != "single quoted command" {
		t.Errorf("Start[0] = %q, want %q", m.Start[0], "single quoted command")
	}
}

func TestParseFile_MissingApp(t *testing.T) {
	path := writeManifest(t, `
start:
  - ./run.sh
`)

	_, err := manifest.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing app, got nil")
	}
}

func TestParseFile_MissingStart(t *testing.T) {
	path := writeManifest(t, `
app: no-start
setup:
  - npm install
`)

	_, err := manifest.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing start, got nil")
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	_, err := manifest.ParseFile("/nonexistent/path/manifest.yml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseFile_UnknownKeysIgnored(t *testing.T) {
	path := writeManifest(t, `
app: svc
unknown_key: some_value
start:
  - ./run.sh
`)

	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("unknown keys should be ignored, got error: %v", err)
	}

	if m.App != "svc" {
		t.Errorf("App = %q, want %q", m.App, "svc")
	}
}

func TestParseFile_MultipleStartCommands(t *testing.T) {
	path := writeManifest(t, `
app: multi
start:
  - ./migrate.sh
  - ./seed.sh
  - pm2 start app.js
`)

	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Start) != 3 {
		t.Errorf("Start len = %d, want 3", len(m.Start))
	}
	if m.Start[2] != "pm2 start app.js" {
		t.Errorf("Start[2] = %q, want %q", m.Start[2], "pm2 start app.js")
	}
}

func TestValidate_EmptyApp(t *testing.T) {
	m := &manifest.Manifest{
		App:   "",
		Start: []string{"./run.sh"},
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for empty app")
	}
}

func TestValidate_EmptyStart(t *testing.T) {
	m := &manifest.Manifest{
		App:   "svc",
		Start: nil,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for empty start")
	}
}

func TestValidate_Valid(t *testing.T) {
	m := &manifest.Manifest{
		App:   "svc",
		Start: []string{"./run.sh"},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
