// Package color provides ANSI-colored terminal output helpers.
// Color is automatically disabled when stdout is not a TTY or when
// the NO_COLOR environment variable is set (https://no-color.org).
// Pass --no-color to force disable at the CLI level.
package color

import (
	"fmt"
	"io"
	"os"
)

// ANSI escape sequences for the colors runway uses.
const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	bold   = "\033[1m"
)

// Enabled controls whether color codes are emitted.
// Set via Init() based on TTY detection; can be forced off via --no-color.
var Enabled = true

// Init detects whether the given writer is a TTY and whether NO_COLOR is set.
// Call once at startup before any output is written.
func Init(w io.Writer, forceDisable bool) {
	if forceDisable || os.Getenv("NO_COLOR") != "" {
		Enabled = false
		return
	}
	Enabled = isTTY(w)
}

// isTTY returns true if w is an *os.File connected to a terminal.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	// ModeCharDevice is set for terminal file descriptors.
	return (info.Mode() & os.ModeCharDevice) != 0
}

func apply(code, s string) string {
	if !Enabled {
		return s
	}
	return code + s + reset
}

// Green returns s formatted in green (used for ✓ success lines).
func Green(s string) string { return apply(green, s) }

// Red returns s formatted in red (used for ✗ error lines).
func Red(s string) string { return apply(red, s) }

// Yellow returns s formatted in yellow (used for warnings).
func Yellow(s string) string { return apply(yellow, s) }

// Cyan returns s formatted in cyan (used for → info/progress lines).
func Cyan(s string) string { return apply(cyan, s) }

// Bold returns s formatted in bold.
func Bold(s string) string { return apply(bold, s) }

// Checkmark returns a green ✓ prefix string.
func Checkmark() string { return Green("✓") }

// Cross returns a red ✗ prefix string.
func Cross() string { return Red("✗") }

// Arrow returns a cyan → prefix string.
func Arrow() string { return Cyan("→") }

// Warn returns a yellow ⚠ prefix string.
func Warn() string { return Yellow("⚠") }

// Successf formats and prints a green ✓ line to w.
func Successf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Checkmark(), fmt.Sprintf(format, a...))
}

// Errorf formats and prints a red ✗ line to w.
func Errorf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Cross(), fmt.Sprintf(format, a...))
}

// Infof formats and prints a cyan → line to w.
func Infof(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Arrow(), fmt.Sprintf(format, a...))
}

// Warnf formats and prints a yellow ⚠ line to w.
func Warnf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Warn(), fmt.Sprintf(format, a...))
}
