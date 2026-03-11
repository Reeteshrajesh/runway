package multiapp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/multiapp"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "runway.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

func TestParseFile_TwoApps(t *testing.T) {
	path := writeConfig(t, `apps:
  - name: api
    repo: git@github.com:org/api.git
    base_dir: /opt/runway/api

  - name: web
    repo: git@github.com:org/web.git
    base_dir: /opt/runway/web
`)
	cfg, err := multiapp.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}

	api := cfg.Apps[0]
	if api.Name != "api" {
		t.Errorf("Apps[0].Name = %q", api.Name)
	}
	if api.Repo != "git@github.com:org/api.git" {
		t.Errorf("Apps[0].Repo = %q", api.Repo)
	}
	if api.BaseDir != "/opt/runway/api" {
		t.Errorf("Apps[0].BaseDir = %q", api.BaseDir)
	}

	web := cfg.Apps[1]
	if web.Name != "web" {
		t.Errorf("Apps[1].Name = %q", web.Name)
	}
}

func TestParseFile_WithBranches(t *testing.T) {
	path := writeConfig(t, `apps:
  - name: api
    repo: git@github.com:org/api.git
    base_dir: /opt/runway/api
    branches:
      - main
      - release/*
`)
	cfg, err := multiapp.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}
	app := cfg.Apps[0]
	if len(app.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(app.Branches), app.Branches)
	}
	if app.Branches[0] != "main" {
		t.Errorf("Branches[0] = %q", app.Branches[0])
	}
	if app.Branches[1] != "release/*" {
		t.Errorf("Branches[1] = %q", app.Branches[1])
	}
}

func TestParseFile_MissingRepo_Error(t *testing.T) {
	path := writeConfig(t, `apps:
  - name: api
    base_dir: /opt/runway/api
`)
	_, err := multiapp.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestParseFile_MissingBaseDir_Error(t *testing.T) {
	path := writeConfig(t, `apps:
  - name: api
    repo: git@github.com:org/api.git
`)
	_, err := multiapp.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing base_dir")
	}
}

func TestParseFile_DuplicateBaseDir_Error(t *testing.T) {
	path := writeConfig(t, `apps:
  - name: api
    repo: git@github.com:org/api.git
    base_dir: /opt/runway/shared

  - name: web
    repo: git@github.com:org/web.git
    base_dir: /opt/runway/shared
`)
	_, err := multiapp.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for duplicate base_dir")
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	_, err := multiapp.ParseFile("/tmp/runway-no-such-file.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAppConfig_BranchAllowed_NoRules(t *testing.T) {
	app := &multiapp.AppConfig{Name: "api", Branches: nil}
	if !app.BranchAllowed("main") {
		t.Error("expected all branches allowed when no rules configured")
	}
}

func TestAppConfig_BranchAllowed_ExactMatch(t *testing.T) {
	app := &multiapp.AppConfig{Name: "api", Branches: []string{"main"}}
	if !app.BranchAllowed("main") {
		t.Error("expected main to be allowed")
	}
	if app.BranchAllowed("develop") {
		t.Error("expected develop to be denied")
	}
}

func TestAppConfig_BranchAllowed_Wildcard(t *testing.T) {
	app := &multiapp.AppConfig{Name: "api", Branches: []string{"release/*"}}
	if !app.BranchAllowed("release/1.0") {
		t.Error("expected release/1.0 to be allowed")
	}
	if app.BranchAllowed("main") {
		t.Error("expected main to be denied")
	}
}

func TestParseFile_CommentsAndBlanks(t *testing.T) {
	path := writeConfig(t, `# runway multi-app config
apps:
  # first app
  - name: api
    repo: git@github.com:org/api.git
    base_dir: /opt/runway/api

  # second app
  - name: worker
    repo: git@github.com:org/worker.git
    base_dir: /opt/runway/worker
`)
	cfg, err := multiapp.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}
}
