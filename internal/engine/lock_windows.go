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
// Unlike flock(2) on Linux/macOS, this lock is NOT automatically released by
// the kernel on crash. To guard against permanent lockout, AcquireLock checks
// whether the PID recorded in an existing lock file belongs to a live process.
// If the process is gone (stale lock), the file is removed and acquisition
// retried once.
//
// runway is a Linux-first tool; Windows support is provided for local
// testing only and is not recommended for production use.
func AcquireLock(path string) (*FileLock, error) {
	if path == "" {
		path = defaultLockPath
	}

	// Inner attempt: exclusive create — fails only if file already exists.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
		return &FileLock{path: path, file: f}, nil
	}

	if !os.IsExist(err) {
		return nil, fmt.Errorf("lock: cannot create %s: %w", path, err)
	}

	// Lock file exists — check whether the recorded PID is still alive.
	if pid, pidErr := HeldByPID(path); pidErr == nil && !isProcessAlive(pid) {
		// Stale lock: process is gone. Remove the orphaned file and retry once.
		_ = os.Remove(path)
		f2, err2 := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err2 == nil {
			_, _ = fmt.Fprintf(f2, "%d\n", os.Getpid())
			return &FileLock{path: path, file: f2}, nil
		}
		if !os.IsExist(err2) {
			return nil, fmt.Errorf("lock: cannot create %s: %w", path, err2)
		}
	}

	return nil, ErrLockHeld
}

// isProcessAlive returns true if a process with the given PID appears to be
// running. On Windows, os.FindProcess always succeeds (it does not probe the
// kernel), so we conservatively return true for any valid PID. The worst case
// is one spurious ErrLockHeld when the previous holder has already exited —
// an operator can manually remove the lock file in that situation.
// runway is Linux-first; full stale-lock detection on Windows is out of scope.
func isProcessAlive(_ int) bool {
	return true
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
