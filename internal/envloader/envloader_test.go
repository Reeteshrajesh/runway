package envloader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Reeteshrajesh/runway/internal/envloader"
)

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	return path
}

func TestLoad_BasicPairs(t *testing.T) {
	path := writeEnvFile(t, `
FOO=bar
BAZ=qux
`)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pairs) != 2 {
		t.Fatalf("len = %d, want 2", len(pairs))
	}
	if pairs[0] != "FOO=bar" {
		t.Errorf("pairs[0] = %q, want %q", pairs[0], "FOO=bar")
	}
	if pairs[1] != "BAZ=qux" {
		t.Errorf("pairs[1] = %q, want %q", pairs[1], "BAZ=qux")
	}
}

func TestLoad_CommentsAndBlankLines(t *testing.T) {
	path := writeEnvFile(t, `
# this is a comment
FOO=bar

# another comment
BAZ=qux
`)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pairs) != 2 {
		t.Errorf("len = %d, want 2 (comments and blanks should be skipped)", len(pairs))
	}
}

func TestLoad_DoubleQuotedValues(t *testing.T) {
	path := writeEnvFile(t, `DB_URL="postgres://localhost/mydb"`)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pairs) != 1 {
		t.Fatalf("len = %d, want 1", len(pairs))
	}
	if pairs[0] != "DB_URL=postgres://localhost/mydb" {
		t.Errorf("pairs[0] = %q, want quotes stripped", pairs[0])
	}
}

func TestLoad_SingleQuotedValues(t *testing.T) {
	path := writeEnvFile(t, `SECRET='my secret value'`)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pairs[0] != "SECRET=my secret value" {
		t.Errorf("pairs[0] = %q, want quotes stripped", pairs[0])
	}
}

func TestLoad_DuplicateKeys_LastWins(t *testing.T) {
	path := writeEnvFile(t, `
PORT=3000
PORT=4000
`)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pairs) != 1 {
		t.Fatalf("len = %d, want 1 (duplicate key should be deduped)", len(pairs))
	}
	if pairs[0] != "PORT=4000" {
		t.Errorf("pairs[0] = %q, want last value PORT=4000", pairs[0])
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	pairs, err := envloader.Load("")
	if err != nil {
		t.Fatalf("empty path should return nil, nil; got error: %v", err)
	}
	if pairs != nil {
		t.Errorf("expected nil pairs for empty path, got %v", pairs)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := envloader.Load("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_MissingEquals(t *testing.T) {
	path := writeEnvFile(t, `INVALID_LINE_NO_EQUALS`)

	_, err := envloader.Load(path)
	if err == nil {
		t.Fatal("expected error for line without '=', got nil")
	}
}

func TestLoad_EmptyKey(t *testing.T) {
	path := writeEnvFile(t, `=value`)

	_, err := envloader.Load(path)
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestLoad_ValueWithEqualsSign(t *testing.T) {
	// Values can contain '=' (e.g. base64 encoded strings).
	path := writeEnvFile(t, `TOKEN=abc=def==`)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pairs[0] != "TOKEN=abc=def==" {
		t.Errorf("pairs[0] = %q, want %q", pairs[0], "TOKEN=abc=def==")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeEnvFile(t, ``)

	pairs, err := envloader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs for empty file, got %d", len(pairs))
	}
}

func TestMerge(t *testing.T) {
	base := []string{"A=1", "B=2"}
	extra := []string{"C=3", "B=override"}

	merged := envloader.Merge(base, extra)

	// merged should contain all 4 entries; exec.Cmd.Env uses last-wins,
	// so B=override should take precedence over B=2.
	if len(merged) != 4 {
		t.Errorf("len = %d, want 4", len(merged))
	}
	if merged[0] != "A=1" {
		t.Errorf("merged[0] = %q, want A=1", merged[0])
	}
	if merged[3] != "B=override" {
		t.Errorf("merged[3] = %q, want B=override", merged[3])
	}
}

func TestMerge_NilLoaded(t *testing.T) {
	base := []string{"A=1"}
	merged := envloader.Merge(base, nil)
	if len(merged) != 1 || merged[0] != "A=1" {
		t.Errorf("merge with nil loaded = %v, want [A=1]", merged)
	}
}
