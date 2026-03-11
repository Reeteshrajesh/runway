package manifest_test

import (
	"testing"

	"github.com/Reeteshrajesh/runway/internal/manifest"
)

func TestBranchAllowed_NoBranches_AllowsAll(t *testing.T) {
	m := &manifest.Manifest{Branches: nil}
	for _, branch := range []string{"main", "feat/foo", "release/1.0", ""} {
		if !m.BranchAllowed(branch) {
			t.Errorf("BranchAllowed(%q) = false, want true when Branches is empty", branch)
		}
	}
}

func TestBranchAllowed_ExactMatch(t *testing.T) {
	m := &manifest.Manifest{Branches: []string{"main"}}

	if !m.BranchAllowed("main") {
		t.Error("BranchAllowed(\"main\") = false, want true")
	}
	if m.BranchAllowed("master") {
		t.Error("BranchAllowed(\"master\") = true, want false")
	}
	if m.BranchAllowed("main-extra") {
		t.Error("BranchAllowed(\"main-extra\") = true, want false (no wildcard)")
	}
}

func TestBranchAllowed_WildcardMatch(t *testing.T) {
	m := &manifest.Manifest{Branches: []string{"release/*"}}

	if !m.BranchAllowed("release/1.0") {
		t.Error("BranchAllowed(\"release/1.0\") = false, want true")
	}
	if !m.BranchAllowed("release/hotfix") {
		t.Error("BranchAllowed(\"release/hotfix\") = false, want true")
	}
	if m.BranchAllowed("main") {
		t.Error("BranchAllowed(\"main\") = true, want false")
	}
	if m.BranchAllowed("feature/release/foo") {
		t.Error("BranchAllowed(\"feature/release/foo\") = true, want false")
	}
}

func TestBranchAllowed_MultiplePatterns(t *testing.T) {
	m := &manifest.Manifest{Branches: []string{"main", "release/*", "hotfix/*"}}

	allowed := []string{"main", "release/1.2", "release/2.0", "hotfix/critical"}
	for _, b := range allowed {
		if !m.BranchAllowed(b) {
			t.Errorf("BranchAllowed(%q) = false, want true", b)
		}
	}

	denied := []string{"master", "feat/foo", "develop"}
	for _, b := range denied {
		if m.BranchAllowed(b) {
			t.Errorf("BranchAllowed(%q) = true, want false", b)
		}
	}
}

func TestBranchAllowed_EmptyBranch_AlwaysAllowed(t *testing.T) {
	// Empty branch string means no branch info (e.g. CLI deploy) — always allow.
	m := &manifest.Manifest{Branches: []string{"main"}}
	if !m.BranchAllowed("") {
		t.Error("BranchAllowed(\"\") = false, want true (CLI deploys bypass branch rules)")
	}
}

func TestBranchAllowed_ParsedFromManifest(t *testing.T) {
	path := writeManifest(t, `app: my-service
start:
  - ./start.sh
branches:
  - main
  - release/*
`)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(m.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(m.Branches), m.Branches)
	}
	if !m.BranchAllowed("main") {
		t.Error("expected main to be allowed")
	}
	if !m.BranchAllowed("release/2.0") {
		t.Error("expected release/2.0 to be allowed")
	}
	if m.BranchAllowed("develop") {
		t.Error("expected develop to be denied")
	}
}
