package release_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Reeteshrajesh/runway/internal/release"
)

func newHistoryManager(t *testing.T) *release.HistoryManager {
	t.Helper()
	return release.NewHistory(t.TempDir())
}

func TestHistory_LoadEmpty(t *testing.T) {
	h := newHistoryManager(t)

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load on missing file should return empty history, got error: %v", err)
	}
	if hist.Current != "" {
		t.Errorf("Current = %q, want empty", hist.Current)
	}
	if len(hist.Deployments) != 0 {
		t.Errorf("Deployments len = %d, want 0", len(hist.Deployments))
	}
}

func TestHistory_AppendSingle(t *testing.T) {
	h := newHistoryManager(t)

	err := h.Append(release.Deployment{
		Commit:    "abc123",
		Time:      time.Now().UTC(),
		Status:    release.StatusRunning,
		Triggered: "cli",
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if hist.Current != "abc123" {
		t.Errorf("Current = %q, want %q", hist.Current, "abc123")
	}
	if len(hist.Deployments) != 1 {
		t.Fatalf("Deployments len = %d, want 1", len(hist.Deployments))
	}
	if hist.Deployments[0].Commit != "abc123" {
		t.Errorf("Deployments[0].Commit = %q, want %q", hist.Deployments[0].Commit, "abc123")
	}
}

func TestHistory_AppendMultiple_NewestFirst(t *testing.T) {
	h := newHistoryManager(t)

	commits := []string{"first", "second", "third"}
	for _, c := range commits {
		if err := h.Append(release.Deployment{
			Commit: c, Time: time.Now().UTC(), Status: release.StatusRunning,
		}); err != nil {
			t.Fatalf("Append %s: %v", c, err)
		}
	}

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Newest (third) should be first.
	if hist.Deployments[0].Commit != "third" {
		t.Errorf("Deployments[0].Commit = %q, want %q", hist.Deployments[0].Commit, "third")
	}
	if hist.Current != "third" {
		t.Errorf("Current = %q, want %q", hist.Current, "third")
	}
}

func TestHistory_PreviousRunningBecomePrevious(t *testing.T) {
	h := newHistoryManager(t)

	_ = h.Append(release.Deployment{
		Commit: "old", Time: time.Now().UTC(), Status: release.StatusRunning,
	})
	_ = h.Append(release.Deployment{
		Commit: "new", Time: time.Now().UTC(), Status: release.StatusRunning,
	})

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// "old" should now be StatusPrevious.
	var oldEntry *release.Deployment
	for i := range hist.Deployments {
		if hist.Deployments[i].Commit == "old" {
			oldEntry = &hist.Deployments[i]
			break
		}
	}
	if oldEntry == nil {
		t.Fatal("'old' deployment not found in history")
	}
	if oldEntry.Status != release.StatusPrevious {
		t.Errorf("old status = %q, want %q", oldEntry.Status, release.StatusPrevious)
	}
}

func TestHistory_MaxReleasesEnforced(t *testing.T) {
	h := newHistoryManager(t)

	// Insert MaxReleases + 5 entries.
	total := release.MaxReleases + 5
	for i := 0; i < total; i++ {
		_ = h.Append(release.Deployment{
			Commit: string(rune('a' + i)),
			Time:   time.Now().UTC(),
			Status: release.StatusSuccess,
		})
	}

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(hist.Deployments) > release.MaxReleases {
		t.Errorf("Deployments len = %d, want <= %d", len(hist.Deployments), release.MaxReleases)
	}
}

func TestHistory_FailedDeployDoesNotUpdateCurrent(t *testing.T) {
	h := newHistoryManager(t)

	_ = h.Append(release.Deployment{
		Commit: "good", Time: time.Now().UTC(), Status: release.StatusRunning,
	})
	_ = h.Append(release.Deployment{
		Commit: "bad", Time: time.Now().UTC(), Status: release.StatusFailed,
	})

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Current should still point to "good", not "bad".
	if hist.Current != "good" {
		t.Errorf("Current = %q, want %q (failed deploy should not update current)", hist.Current, "good")
	}
}

func TestHistory_RolledBackUpdatesCurrent(t *testing.T) {
	h := newHistoryManager(t)

	_ = h.Append(release.Deployment{
		Commit: "v1", Time: time.Now().UTC(), Status: release.StatusRunning,
	})
	_ = h.Append(release.Deployment{
		Commit: "v2", Time: time.Now().UTC(), Status: release.StatusRunning,
	})
	// Rollback to v1.
	_ = h.Append(release.Deployment{
		Commit: "v1", Time: time.Now().UTC(), Status: release.StatusRolledBack,
	})

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if hist.Current != "v1" {
		t.Errorf("Current = %q, want %q after rollback", hist.Current, "v1")
	}
}

func TestHistory_AtomicWrite_FileIsValid(t *testing.T) {
	h := newHistoryManager(t)

	// Multiple rapid writes should leave a valid JSON file each time.
	for i := 0; i < 10; i++ {
		_ = h.Append(release.Deployment{
			Commit: "commit" + string(rune('0'+i)),
			Time:   time.Now().UTC(),
			Status: release.StatusRunning,
		})

		// Reload and verify parseable after every write.
		if _, err := h.Load(); err != nil {
			t.Fatalf("Load after write %d: %v", i, err)
		}
	}
}

// TestHistory_BackupCreatedOnSave verifies that Append writes history.json.bak
// alongside history.json so a recovery copy always exists after the first save.
func TestHistory_BackupCreatedOnSave(t *testing.T) {
	dir := t.TempDir()
	h := release.NewHistory(dir)

	if err := h.Append(release.Deployment{
		Commit: "abc123", Time: time.Now().UTC(), Status: release.StatusRunning,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// After the second save the backup must exist (first save has nothing to back up).
	if err := h.Append(release.Deployment{
		Commit: "def456", Time: time.Now().UTC(), Status: release.StatusSuccess,
	}); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	bakPath := filepath.Join(dir, "history.json.bak")
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("expected history.json.bak to exist after second save, got: %v", err)
	}
}

// TestHistory_RecoveryFromBackup verifies that Load falls back to history.json.bak
// when history.json is corrupt, and restores it as the live file.
func TestHistory_RecoveryFromBackup(t *testing.T) {
	dir := t.TempDir()
	h := release.NewHistory(dir)

	// First save — no backup yet (nothing to back up on initial write).
	if err := h.Append(release.Deployment{
		Commit: "good-commit", Time: time.Now().UTC(), Status: release.StatusRunning,
	}); err != nil {
		t.Fatalf("Append 1: %v", err)
	}

	// Second save — this writes history.json.bak containing the first save's data.
	if err := h.Append(release.Deployment{
		Commit: "good-commit", Time: time.Now().UTC(), Status: release.StatusSuccess,
	}); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	// Now corrupt the live file — the backup still holds good data.
	histPath := filepath.Join(dir, "history.json")
	if err := os.WriteFile(histPath, []byte("{broken json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Load should recover from the backup without returning an error.
	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load after corruption: %v", err)
	}
	if hist.Current != "good-commit" {
		t.Errorf("Current = %q after recovery, want %q", hist.Current, "good-commit")
	}

	// The live file should now be valid JSON again (restored from backup).
	if _, err := h.Load(); err != nil {
		t.Errorf("second Load after recovery: %v", err)
	}
}

// TestHistory_EmptyHistoryWhenBothCorrupt verifies that Load returns an empty
// history (rather than an error) when both history.json and the backup are corrupt.
func TestHistory_EmptyHistoryWhenBothCorrupt(t *testing.T) {
	dir := t.TempDir()
	h := release.NewHistory(dir)

	histPath := filepath.Join(dir, "history.json")
	bakPath := filepath.Join(dir, "history.json.bak")

	if err := os.WriteFile(histPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile hist: %v", err)
	}
	if err := os.WriteFile(bakPath, []byte("also not json"), 0644); err != nil {
		t.Fatalf("WriteFile bak: %v", err)
	}

	hist, err := h.Load()
	if err != nil {
		t.Fatalf("Load should not error when both files are corrupt, got: %v", err)
	}
	if len(hist.Deployments) != 0 {
		t.Errorf("expected empty deployments, got %d", len(hist.Deployments))
	}
}
