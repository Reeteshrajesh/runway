package color_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/color"
)

func TestColorDisabled_NoAnsiCodes(t *testing.T) {
	// A bytes.Buffer is not a TTY, so Init should disable color.
	var buf bytes.Buffer
	color.Init(&buf, false)

	if color.Enabled {
		// Force disable for test predictability regardless of test runner TTY.
		color.Enabled = false
	}

	if got := color.Green("ok"); strings.Contains(got, "\033[") {
		t.Errorf("Green() emitted ANSI codes when color is disabled: %q", got)
	}
	if got := color.Red("fail"); strings.Contains(got, "\033[") {
		t.Errorf("Red() emitted ANSI codes when color is disabled: %q", got)
	}
}

func TestForceDisable(t *testing.T) {
	var buf bytes.Buffer
	color.Init(&buf, true) // forceDisable=true
	if color.Enabled {
		t.Error("Enabled should be false when forceDisable=true")
	}
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	color.Init(&buf, false)
	if color.Enabled {
		t.Error("Enabled should be false when NO_COLOR env is set")
	}
}

func TestHelpers_ReturnText(t *testing.T) {
	color.Enabled = false
	for _, tc := range []struct {
		fn func(string) string
		in string
	}{
		{color.Green, "success"},
		{color.Red, "error"},
		{color.Yellow, "warn"},
		{color.Cyan, "info"},
		{color.Bold, "bold"},
	} {
		if got := tc.fn(tc.in); got != tc.in {
			t.Errorf("expected %q, got %q (color disabled)", tc.in, got)
		}
	}
}

func TestHelpers_EmitAnsiWhenEnabled(t *testing.T) {
	color.Enabled = true
	if got := color.Green("ok"); !strings.Contains(got, "\033[") {
		t.Errorf("Green() should emit ANSI when enabled, got %q", got)
	}
	if got := color.Red("x"); !strings.Contains(got, "\033[") {
		t.Errorf("Red() should emit ANSI when enabled, got %q", got)
	}
	color.Enabled = false // reset
}

func TestPrintFunctions_WriteToWriter(t *testing.T) {
	color.Enabled = false
	var buf bytes.Buffer
	color.Successf(&buf, "done %s", "here")
	color.Errorf(&buf, "oops")
	color.Infof(&buf, "deploying")
	color.Warnf(&buf, "careful")

	out := buf.String()
	for _, want := range []string{"done here", "oops", "deploying", "careful"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}
