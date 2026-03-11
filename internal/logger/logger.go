// Package logger provides a per-deployment file logger.
//
// Each deployment writes its full stdout and stderr to a deploy.log file
// inside its release directory. The logger implements io.Writer so it can
// be passed directly to os/exec Cmd.Stdout and Cmd.Stderr.
package logger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const logFileName = "deploy.log"

// DeployLogger writes timestamped log lines to a deploy.log file.
// It implements io.Writer and io.Closer.
type DeployLogger struct {
	path   string
	file   *os.File
	writer *bufio.Writer
	// tee mirrors all output to an additional writer (e.g. os.Stdout for CLI feedback).
	tee     io.Writer
	colored bool // true when tee is a TTY and color output is enabled
}

// New creates a new DeployLogger that writes to <releaseDir>/deploy.log.
// If tee is non-nil, output is mirrored to it in addition to the file.
func New(releaseDir string, tee io.Writer) (*DeployLogger, error) {
	path := filepath.Join(releaseDir, logFileName)

	// 0640: owner rw, group r — deploy.log should not be world-readable
	// (it may contain env var names or command output with sensitive paths).
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return nil, fmt.Errorf("logger: cannot open %s: %w", path, err)
	}

	return &DeployLogger{
		path:   path,
		file:   f,
		writer: bufio.NewWriter(f),
		tee:    tee,
	}, nil
}

// Write implements io.Writer. Raw bytes (e.g. command output) are written
// directly to the log file without additional timestamping.
func (l *DeployLogger) Write(p []byte) (n int, err error) {
	n, err = l.writer.Write(p)
	if err != nil {
		return n, err
	}
	if l.tee != nil {
		// Best-effort mirror to tee; ignore errors so a broken tee
		// does not abort the deployment.
		_, _ = l.tee.Write(p)
	}
	return n, nil
}

// Logf writes a formatted, timestamped line to the log.
// Use this for runway's own status messages (not command output).
func (l *DeployLogger) Logf(format string, args ...any) {
	ts := time.Now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] %s\n", ts, fmt.Sprintf(format, args...))
	_, _ = l.writer.WriteString(line)
	if l.tee != nil {
		_, _ = fmt.Fprint(l.tee, line)
	}
}

// Path returns the absolute path of the log file.
func (l *DeployLogger) Path() string {
	return l.path
}

// Close flushes the buffer and closes the underlying file.
// Always call Close (via defer) after creating a DeployLogger.
func (l *DeployLogger) Close() error {
	if err := l.writer.Flush(); err != nil {
		_ = l.file.Close()
		return fmt.Errorf("logger: flush %s: %w", l.path, err)
	}
	return l.file.Close()
}

// linePrefixWriter wraps an io.Writer and prepends prefix to each line.
// Partial lines (no trailing newline) are held until the newline arrives.
type linePrefixWriter struct {
	w      io.Writer
	prefix string
	buf    []byte
}

func (lp *linePrefixWriter) Write(p []byte) (int, error) {
	total := len(p)
	lp.buf = append(lp.buf, p...)
	for {
		idx := -1
		for i, b := range lp.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		line := lp.buf[:idx+1]
		_, err := fmt.Fprintf(lp.w, "%s%s", lp.prefix, line)
		if err != nil {
			return 0, err
		}
		lp.buf = lp.buf[idx+1:]
	}
	return total, nil
}

// NewStreaming creates a DeployLogger whose tee output is prefixed with
// "[runway] " on each line. The log file receives raw unprefixed output.
// Use this for CLI-triggered deploys where the operator is watching the terminal.
func NewStreaming(releaseDir string, prefix string, out io.Writer) (*DeployLogger, error) {
	tee := &linePrefixWriter{w: out, prefix: prefix}
	l, err := New(releaseDir, tee)
	if err != nil {
		return nil, err
	}
	// Enable color if out is a TTY.
	if f, ok := out.(*os.File); ok {
		if info, statErr := f.Stat(); statErr == nil {
			l.colored = (info.Mode() & os.ModeCharDevice) != 0
		}
	}
	return l, nil
}

// Step prints a step banner directly to the terminal tee writer.
// When the tee is a TTY, the arrow prefix is rendered in cyan.
// Step does NOT write to the log file — use Logf for file-level step markers.
func (l *DeployLogger) Step(format string, args ...any) {
	if l.tee == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.colored {
		// Cyan arrow + reset
		_, _ = fmt.Fprintf(l.tee, "\033[36m%s\033[0m\n", msg)
	} else {
		_, _ = fmt.Fprintf(l.tee, "%s\n", msg)
	}
}
