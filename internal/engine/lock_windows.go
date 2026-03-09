//go:build windows

package engine

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const defaultLockPath = `C:\Windows\Temp\runway.lock`

// ErrLockHeld is returned when a deployment is already in progress.
var ErrLockHeld = errors.New("a deployment is already in progress — try again later")

// FileLock represents an acquired exclusive file lock.
// On Windows this is a best-effort PID-file lock (no kernel-level flock).
type FileLock struct {
	path string
	file *os.File
}

// AcquireLock acquires a best-effort lock on Windows using a PID file.
// Note: unlike flock(2) on Linux/macOS, this lock is NOT automatically
// released by the kernel on crash — it requires manual cleanup.
// runway is a Linux-first tool; Windows support is provided for local
// testing only and is not recommended for production use.
func AcquireLock(path string) (*FileLock, error) {
	if path == "" {
		path = defaultLockPath
	}

	// Try exclusive create — fails if file already exists.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrLockHeld
		}
		return nil, fmt.Errorf("lock: cannot create %s: %w", path, err)
	}

	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return &FileLock{path: path, file: f}, nil
}

// Release removes the lock file.
func (l *FileLock) Release() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("lock: close %s: %w", l.path, err)
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lock: remove %s: %w", l.path, err)
	}
	return nil
}

// HeldByPID reads the PID from the lock file.
func HeldByPID(path string) (int, error) {
	if path == "" {
		path = defaultLockPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(string(data[:len(data)-1]))
	if err != nil {
		return 0, fmt.Errorf("lock: invalid PID in lock file: %w", err)
	}
	return pid, nil
}
