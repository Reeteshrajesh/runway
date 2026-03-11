package webhook

import (
	"testing"
)

func TestExtractBranch_MainRef(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)
	got := extractBranch(body)
	if got != "main" {
		t.Errorf("extractBranch = %q, want %q", got, "main")
	}
}

func TestExtractBranch_FeatureRef(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/feat/my-feature","after":"abc123"}`)
	got := extractBranch(body)
	if got != "feat/my-feature" {
		t.Errorf("extractBranch = %q, want %q", got, "feat/my-feature")
	}
}

func TestExtractBranch_TagRef_ReturnsEmpty(t *testing.T) {
	// Tags have ref "refs/tags/v1.0" — not a branch.
	body := []byte(`{"ref":"refs/tags/v1.0","after":"abc123"}`)
	got := extractBranch(body)
	if got != "" {
		t.Errorf("extractBranch for tag ref = %q, want \"\"", got)
	}
}

func TestExtractBranch_MissingRef_ReturnsEmpty(t *testing.T) {
	body := []byte(`{"after":"abc123"}`)
	got := extractBranch(body)
	if got != "" {
		t.Errorf("extractBranch with no ref = %q, want \"\"", got)
	}
}

func TestExtractBranch_InvalidJSON_ReturnsEmpty(t *testing.T) {
	got := extractBranch([]byte(`not json`))
	if got != "" {
		t.Errorf("extractBranch on bad JSON = %q, want \"\"", got)
	}
}
