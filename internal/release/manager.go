// Package release manages the releases directory, current symlink, and history.
package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	releasesDir = "releases"
	currentLink = "current"
)

// Manager handles release directory lifecycle and symlink management.
type Manager struct {
	baseDir string
}

// NewManager returns a Manager rooted at baseDir (e.g. /opt/runway).
func NewManager(baseDir string) *Manager {
	return &Manager{baseDir: baseDir}
}

// ReleaseDir returns the absolute path for a specific commit's release directory.
func (m *Manager) ReleaseDir(commit string) string {
	return filepath.Join(m.baseDir, releasesDir, commit)
}

// CurrentLink returns the absolute path of the "current" symlink.
func (m *Manager) CurrentLink() string {
	return filepath.Join(m.baseDir, currentLink)
}

// CreateReleaseDir creates the release directory for a commit.
// Returns an error if the directory already exists (duplicate deploy guard).
func (m *Manager) CreateReleaseDir(commit string) error {
	dir := m.ReleaseDir(commit)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("release %q already exists at %s", commit, dir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create release dir %s: %w", dir, err)
	}
	return nil
}

// RemoveReleaseDir deletes the release directory for a commit.
// Used to clean up after a failed build — never leaves a broken release on disk.
// Safe to call even if the directory does not exist.
func (m *Manager) RemoveReleaseDir(commit string) error {
	dir := m.ReleaseDir(commit)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove release dir %s: %w", dir, err)
	}
	return nil
}

// UpdateCurrent atomically updates the "current" symlink to point to the
// release directory for the given commit.
//
// Atomicity is achieved by creating a temporary symlink adjacent to "current"
// and then using os.Rename to swap it in. On Linux, rename(2) is atomic for
// symlinks, so there is no moment where "current" is missing or broken.
func (m *Manager) UpdateCurrent(commit string) error {
	target := filepath.Join(releasesDir, commit) // relative target for the symlink
	currentPath := m.CurrentLink()
	tmpPath := currentPath + ".new"

	// Remove any stale tmp symlink from a previous crashed deploy.
	_ = os.Remove(tmpPath)

	if err := os.Symlink(target, tmpPath); err != nil {
		return fmt.Errorf("create tmp symlink: %w", err)
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic symlink swap: %w", err)
	}

	return nil
}

// ActiveCommit reads the "current" symlink and returns the commit SHA it points to.
func (m *Manager) ActiveCommit() (string, error) {
	target, err := os.Readlink(m.CurrentLink())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // no active release yet
		}
		return "", fmt.Errorf("read current symlink: %w", err)
	}

	// target is "releases/<commit>" — extract the commit part.
	commit := filepath.Base(target)
	return commit, nil
}

// ListReleases returns all release commit SHAs sorted by directory modification
// time (oldest first). The active release is included.
func (m *Manager) ListReleases() ([]string, error) {
	relDir := filepath.Join(m.baseDir, releasesDir)

	entries, err := os.ReadDir(relDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list releases: %w", err)
	}

	type releaseEntry struct {
		name    string
		modTime int64
	}

	var items []releaseEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, releaseEntry{
			name:    e.Name(),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime < items[j].modTime // oldest first
	})

	commits := make([]string, len(items))
	for i, item := range items {
		commits[i] = item.name
	}
	return commits, nil
}

// Cleanup removes the oldest release directories when the total exceeds MaxReleases.
// The active commit is never removed.
func (m *Manager) Cleanup(activeCommit string) error {
	releases, err := m.ListReleases()
	if err != nil {
		return err
	}

	if len(releases) <= MaxReleases {
		return nil
	}

	// Remove oldest first until we are within the limit.
	toRemove := releases[:len(releases)-MaxReleases]
	var errs []string
	for _, commit := range toRemove {
		if strings.EqualFold(commit, activeCommit) {
			continue // never delete the active release
		}
		if err := m.RemoveReleaseDir(commit); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
