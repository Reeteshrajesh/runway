//go:build linux || darwin

// Package engine contains the core deployment orchestration logic.
package engine

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
)

const defaultLockPath = "/tmp/runway.lock"

// ErrLockHeld is returned when a deployment is already in progress.
var ErrLockHeld = errors.New("a deployment is already in progress — try again later")

// FileLock represents an acquired exclusive file lock.
type FileLock struct {
	path string
	file *os.File
}

// AcquireLock attempts to obtain a non-blocking exclusive lock on the lock file.
// Returns ErrLockHeld if another deploy is already running.
//
// Uses syscall.Flock (LOCK_EX|LOCK_NB) — the kernel automatically releases
// the flock if the holder process crashes, so permanent lockout is impossible.
// The stale-lock check below is therefore a defence-in-depth diagnostic: if
// flock says the lock is held but kill(pid, 0) returns ESRCH, something very
// unexpected has happened (e.g. pid wrap-around or a corrupted lock file) and
// we surface a clear error rather than silently blocking.
func AcquireLock(path string) (*FileLock, error) {
	if path == "" {
		path = defaultLockPath
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("lock: cannot open %s: %w", path, err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			// Lock is genuinely held. Read the PID for a better error message.
			if pid, pidErr := HeldByPID(path); pidErr == nil {
				if !isProcessAlive(pid) {
					// Kernel says locked but process is gone — should not happen
					// with flock, but report clearly instead of blocking forever.
					return nil, fmt.Errorf("lock: held by stale PID %d (process not found); remove %s manually", pid, path)
				}
				return nil, fmt.Errorf("%w (held by PID %d)", ErrLockHeld, pid)
			}
			return nil, ErrLockHeld
		}
		return nil, fmt.Errorf("lock: flock %s: %w", path, err)
	}

	// Write current PID so operators can identify a stuck deploy.
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())

	return &FileLock{path: path, file: f}, nil
}

// isProcessAlive reports whether a process with the given PID is running.
// Uses kill(pid, 0) — sends no signal but checks process existence.
// Returns false only when the kernel returns ESRCH (no such process).
func isProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Release unlocks and removes the lock file.
// Always call Release (via defer) after a successful AcquireLock.
func (l *FileLock) Release() error {
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("lock: unlock %s: %w", l.path, err)
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("lock: close %s: %w", l.path, err)
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lock: remove %s: %w", l.path, err)
	}
	return nil
}

// HeldByPID reads the PID written in the lock file, if any.
// Useful for diagnostics / the status command.
func HeldByPID(path string) (int, error) {
	if path == "" {
		path = defaultLockPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(string(data[:len(data)-1])) // trim newline
	if err != nil {
		return 0, fmt.Errorf("lock: invalid PID in lock file: %w", err)
	}
	return pid, nil
}
