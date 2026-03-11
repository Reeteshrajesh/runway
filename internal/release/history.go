package release

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// MaxReleases is the maximum number of releases kept on disk.
	MaxReleases = 15

	historyFile    = "history.json"
	historyFileBak = "history.json.bak"
)

// DeployStatus describes the outcome of a deployment.
type DeployStatus string

const (
	StatusRunning    DeployStatus = "running"
	StatusSuccess    DeployStatus = "success"
	StatusFailed     DeployStatus = "failed"
	StatusPrevious   DeployStatus = "previous"
	StatusRolledBack DeployStatus = "rolled_back"
)

// Deployment records a single deployment event.
type Deployment struct {
	Commit    string       `json:"commit"`
	Time      time.Time    `json:"time"`
	Status    DeployStatus `json:"status"`
	Triggered string       `json:"triggered"` // "cli" | "webhook"
}

// History represents the deployment history file.
type History struct {
	Current     string       `json:"current"`
	Deployments []Deployment `json:"deployments"`
}

// HistoryManager reads and writes history.json.
type HistoryManager struct {
	path string
}

// NewHistory returns a HistoryManager for the given base directory.
func NewHistory(baseDir string) *HistoryManager {
	return &HistoryManager{
		path: filepath.Join(baseDir, historyFile),
	}
}

// Load reads and parses history.json.
// Returns an empty History if the file does not exist yet (first deploy).
// If history.json is corrupt (invalid JSON), Load automatically falls back to
// history.json.bak. If the backup is also absent or corrupt, Load returns an
// empty History so the next deploy can start fresh rather than being blocked.
func (h *HistoryManager) Load() (*History, error) {
	hist, err := h.loadFile(h.path)
	if err == nil {
		return hist, nil
	}

	// File simply doesn't exist yet — normal on first deploy, no warning needed.
	if errors.Is(err, errHistoryNotFound) {
		return &History{}, nil
	}

	// Primary file is corrupt or unreadable — try the backup.
	bakPath := filepath.Join(filepath.Dir(h.path), historyFileBak)
	bak, bakErr := h.loadFile(bakPath)
	if bakErr == nil {
		// Restore the backup as the live file so future loads don't need to fall back.
		_ = os.Rename(bakPath, h.path)
		fmt.Fprintf(os.Stderr, "history: WARNING: recovered from backup (%s was corrupt: %v)\n", h.path, err)
		return bak, nil
	}

	// Both corrupt — start with an empty history rather than blocking deploys.
	fmt.Fprintf(os.Stderr, "history: WARNING: starting with empty history (%s corrupt: %v)\n", h.path, err)
	return &History{}, nil
}

// errHistoryNotFound is returned by loadFile when the file simply does not exist.
// Distinct from a corruption error so Load can suppress noisy warnings.
var errHistoryNotFound = fmt.Errorf("history file not found")

// loadFile reads and JSON-decodes a single history file path.
func (h *HistoryManager) loadFile(path string) (*History, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errHistoryNotFound
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var hist History
	if err := json.Unmarshal(data, &hist); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &hist, nil
}

// Append adds a new deployment record and saves history.json atomically.
// Marks all previous "running" entries as "previous".
// Enforces the MaxReleases cap — oldest records beyond the cap are dropped.
func (h *HistoryManager) Append(d Deployment) error {
	hist, err := h.Load()
	if err != nil {
		return err
	}

	// Transition any currently "running" record to "previous".
	for i := range hist.Deployments {
		if hist.Deployments[i].Status == StatusRunning {
			hist.Deployments[i].Status = StatusPrevious
		}
	}

	// Prepend new deployment (newest first).
	hist.Deployments = append([]Deployment{d}, hist.Deployments...)

	// Enforce cap.
	if len(hist.Deployments) > MaxReleases {
		hist.Deployments = hist.Deployments[:MaxReleases]
	}

	// Update the "current" pointer only on success.
	if d.Status == StatusRunning || d.Status == StatusRolledBack {
		hist.Current = d.Commit
	}

	return h.save(hist)
}

// save writes history atomically: write to a temp file then rename.
// Before replacing the live file, the current history.json is copied to
// history.json.bak so that Load can recover if the rename is interrupted.
func (h *HistoryManager) save(hist *History) error {
	data, err := json.MarshalIndent(hist, "", "  ")
	if err != nil {
		return fmt.Errorf("history: marshal: %w", err)
	}

	// Back up the current live file before overwriting it.
	// Best-effort: a missing live file (first deploy) is fine.
	bakPath := filepath.Join(filepath.Dir(h.path), historyFileBak)
	if current, readErr := os.ReadFile(h.path); readErr == nil {
		_ = os.WriteFile(bakPath, current, 0644)
	}

	tmpPath := h.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("history: write tmp: %w", err)
	}

	if err := os.Rename(tmpPath, h.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("history: atomic rename: %w", err)
	}

	return nil
}
