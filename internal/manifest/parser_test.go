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

func TestParse_NotifyEmailBlock(t *testing.T) {
	content := `app: myapp
start:
  - systemctl restart myapp
notify:
  email:
    to: team@company.com
    from: runway@server.com
    smtp_host: smtp.gmail.com
    smtp_port: 587
`
	path := writeManifest(t, content)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if m.Notify.To != "team@company.com" {
		t.Errorf("Notify.To = %q, want %q", m.Notify.To, "team@company.com")
	}
	if m.Notify.From != "runway@server.com" {
		t.Errorf("Notify.From = %q, want %q", m.Notify.From, "runway@server.com")
	}
	if m.Notify.SMTPHost != "smtp.gmail.com" {
		t.Errorf("Notify.SMTPHost = %q, want %q", m.Notify.SMTPHost, "smtp.gmail.com")
	}
	if m.Notify.SMTPPort != "587" {
		t.Errorf("Notify.SMTPPort = %q, want %q", m.Notify.SMTPPort, "587")
	}
}

func TestParse_NotifyMissing_NoError(t *testing.T) {
	content := `app: myapp
start:
  - systemctl restart myapp
`
	path := writeManifest(t, content)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if m.Notify.To != "" || m.Notify.SMTPHost != "" {
		t.Errorf("expected empty Notify when not configured, got %+v", m.Notify)
	}
}

func TestParse_PreDeployHooks(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - systemctl restart my-service
pre_deploy:
  - ./scripts/pre-check.sh
  - echo pre-deploy running
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(m.PreDeploy) != 2 {
		t.Fatalf("expected 2 pre_deploy commands, got %d", len(m.PreDeploy))
	}
	if m.PreDeploy[0] != "./scripts/pre-check.sh" {
		t.Errorf("PreDeploy[0] = %q, want %q", m.PreDeploy[0], "./scripts/pre-check.sh")
	}
	if m.PreDeploy[1] != "echo pre-deploy running" {
		t.Errorf("PreDeploy[1] = %q, want %q", m.PreDeploy[1], "echo pre-deploy running")
	}
}

func TestParse_PostDeployHooks(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - systemctl restart my-service
post_deploy:
  - curl -s http://localhost:8080/health
  - ./scripts/notify-slack.sh
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(m.PostDeploy) != 2 {
		t.Fatalf("expected 2 post_deploy commands, got %d", len(m.PostDeploy))
	}
	if m.PostDeploy[0] != "curl -s http://localhost:8080/health" {
		t.Errorf("PostDeploy[0] = %q", m.PostDeploy[0])
	}
}

func TestParse_BothHooks(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - ./start.sh
pre_deploy:
  - ./pre.sh
post_deploy:
  - ./post.sh
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(m.PreDeploy) != 1 || m.PreDeploy[0] != "./pre.sh" {
		t.Errorf("unexpected PreDeploy: %v", m.PreDeploy)
	}
	if len(m.PostDeploy) != 1 || m.PostDeploy[0] != "./post.sh" {
		t.Errorf("unexpected PostDeploy: %v", m.PostDeploy)
	}
}

func TestParse_NoHooks_FieldsEmpty(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - ./start.sh
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(m.PreDeploy) != 0 {
		t.Errorf("expected empty PreDeploy, got %v", m.PreDeploy)
	}
	if len(m.PostDeploy) != 0 {
		t.Errorf("expected empty PostDeploy, got %v", m.PostDeploy)
	}
}

func TestParse_HealthCheck(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - ./start.sh
health_check:
  url: http://localhost:8080/health
  interval: 3
  retries: 5
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if m.HealthCheck.URL != "http://localhost:8080/health" {
		t.Errorf("HealthCheck.URL = %q", m.HealthCheck.URL)
	}
	if m.HealthCheck.Interval != 3 {
		t.Errorf("HealthCheck.Interval = %d, want 3", m.HealthCheck.Interval)
	}
	if m.HealthCheck.Retries != 5 {
		t.Errorf("HealthCheck.Retries = %d, want 5", m.HealthCheck.Retries)
	}
}

func TestParse_HealthCheck_Missing_FieldsZero(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - ./start.sh
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if m.HealthCheck.URL != "" {
		t.Errorf("expected empty HealthCheck.URL, got %q", m.HealthCheck.URL)
	}
}
