package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// MaxReleases is the maximum number of releases kept on disk.
	MaxReleases = 15

	historyFile = "history.json"
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
// Returns an empty History if the file does not exist yet.
func (h *HistoryManager) Load() (*History, error) {
	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, fmt.Errorf("history: read %s: %w", h.path, err)
	}

	var hist History
	if err := json.Unmarshal(data, &hist); err != nil {
		return nil, fmt.Errorf("history: parse %s: %w", h.path, err)
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
// This prevents a partial write from corrupting history.json.
func (h *HistoryManager) save(hist *History) error {
	data, err := json.MarshalIndent(hist, "", "  ")
	if err != nil {
		return fmt.Errorf("history: marshal: %w", err)
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
