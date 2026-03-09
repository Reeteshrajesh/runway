package release_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/release"
)

func newManager(t *testing.T) (*release.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	// Pre-create the releases/ subdirectory (engine does this in production).
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0755); err != nil {
		t.Fatalf("mkdir releases: %v", err)
	}
	return release.NewManager(dir), dir
}

func TestManager_CreateAndRemoveReleaseDir(t *testing.T) {
	mgr, _ := newManager(t)

	if err := mgr.CreateReleaseDir("abc123"); err != nil {
		t.Fatalf("CreateReleaseDir: %v", err)
	}

	dir := mgr.ReleaseDir("abc123")
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("release dir should exist: %v", err)
	}

	if err := mgr.RemoveReleaseDir("abc123"); err != nil {
		t.Fatalf("RemoveReleaseDir: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("release dir should not exist after removal")
	}
}

func TestManager_CreateReleaseDir_Duplicate(t *testing.T) {
	mgr, _ := newManager(t)

	_ = mgr.CreateReleaseDir("abc123")
	err := mgr.CreateReleaseDir("abc123")
	if err == nil {
		t.Fatal("expected error when creating duplicate release dir, got nil")
	}
}

func TestManager_RemoveReleaseDir_Nonexistent(t *testing.T) {
	mgr, _ := newManager(t)

	// Should not return an error for a non-existent directory.
	if err := mgr.RemoveReleaseDir("doesnotexist"); err != nil {
		t.Errorf("RemoveReleaseDir on non-existent should not error: %v", err)
	}
}

func TestManager_UpdateCurrent_Atomic(t *testing.T) {
	mgr, _ := newManager(t)

	_ = mgr.CreateReleaseDir("v1")
	_ = mgr.CreateReleaseDir("v2")

	if err := mgr.UpdateCurrent("v1"); err != nil {
		t.Fatalf("UpdateCurrent v1: %v", err)
	}

	active, err := mgr.ActiveCommit()
	if err != nil {
		t.Fatalf("ActiveCommit: %v", err)
	}
	if active != "v1" {
		t.Errorf("active = %q, want %q", active, "v1")
	}

	// Switch to v2.
	if err := mgr.UpdateCurrent("v2"); err != nil {
		t.Fatalf("UpdateCurrent v2: %v", err)
	}

	active, err = mgr.ActiveCommit()
	if err != nil {
		t.Fatalf("ActiveCommit after switch: %v", err)
	}
	if active != "v2" {
		t.Errorf("active = %q, want %q", active, "v2")
	}
}

func TestManager_ActiveCommit_NoSymlink(t *testing.T) {
	mgr, _ := newManager(t)

	active, err := mgr.ActiveCommit()
	if err != nil {
		t.Fatalf("ActiveCommit with no symlink should return empty string, got error: %v", err)
	}
	if active != "" {
		t.Errorf("active = %q, want empty when no symlink exists", active)
	}
}

func TestManager_ListReleases(t *testing.T) {
	mgr, _ := newManager(t)

	commits := []string{"aaa", "bbb", "ccc"}
	for _, c := range commits {
		if err := mgr.CreateReleaseDir(c); err != nil {
			t.Fatalf("CreateReleaseDir %s: %v", c, err)
		}
	}

	releases, err := mgr.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}

	if len(releases) != 3 {
		t.Errorf("len = %d, want 3", len(releases))
	}
}

func TestManager_ListReleases_Empty(t *testing.T) {
	mgr, _ := newManager(t)

	releases, err := mgr.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases on empty dir: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected 0 releases, got %d", len(releases))
	}
}

func TestManager_Cleanup_RemovesOldest(t *testing.T) {
	mgr, _ := newManager(t)

	// Create MaxReleases + 3 release dirs.
	total := release.MaxReleases + 3
	var active string
	for i := 0; i < total; i++ {
		commit := string(rune('a'+i)) + "commit"
		_ = mgr.CreateReleaseDir(commit)
		active = commit
	}

	if err := mgr.Cleanup(active); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	releases, err := mgr.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases after cleanup: %v", err)
	}

	if len(releases) > release.MaxReleases {
		t.Errorf("after cleanup len = %d, want <= %d", len(releases), release.MaxReleases)
	}
}

func TestManager_Cleanup_DoesNotRemoveActive(t *testing.T) {
	mgr, _ := newManager(t)

	// Create exactly MaxReleases + 1 releases.
	commits := make([]string, release.MaxReleases+1)
	for i := range commits {
		commits[i] = string(rune('a'+i)) + "x"
		_ = mgr.CreateReleaseDir(commits[i])
	}
	// The first commit (oldest) is what we declare as active.
	active := commits[0]

	if err := mgr.Cleanup(active); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	// Active should still be on disk.
	if _, err := os.Stat(mgr.ReleaseDir(active)); err != nil {
		t.Errorf("active release %q was removed during cleanup", active)
	}
}
