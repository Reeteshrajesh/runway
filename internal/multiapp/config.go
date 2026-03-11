// Package multiapp parses runway.yml — the optional multi-app configuration
// file. When runway.yml exists in GITOPS_DIR, a single runway instance can
// manage multiple applications.
//
// runway.yml example:
//
//	apps:
//	  - name: api
//	    repo: git@github.com:org/api.git
//	    base_dir: /opt/runway/api
//	    branches:
//	      - main
//	      - release/*
//
//	  - name: web
//	    repo: git@github.com:org/web.git
//	    base_dir: /opt/runway/web
//	    branches:
//	      - main
package multiapp

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AppConfig holds the configuration for one app inside runway.yml.
type AppConfig struct {
	// Name is a human-readable identifier for this app (required).
	Name string

	// Repo is the git repository URL for this app (required).
	Repo string

	// BaseDir is the working directory for this app (required).
	// Each app must have a unique base_dir.
	BaseDir string

	// Branches is the list of branch patterns allowed to deploy this app.
	// Same wildcard semantics as manifest.yml branches:.
	// Empty = all branches allowed.
	Branches []string
}

// Config holds the full parsed contents of runway.yml.
type Config struct {
	Apps []AppConfig
}

// ParseFile reads and parses runway.yml at the given path.
// Returns an error if the file cannot be opened or is structurally invalid.
// Unknown keys are silently ignored for forward compatibility.
func ParseFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("multiapp: cannot open %s: %w", path, err)
	}
	defer f.Close()

	cfg := &Config{}
	var current *AppConfig // pointer into cfg.Apps slice element

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		raw := sc.Text()
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		// Skip blanks and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Top-level "apps:" opener.
		if trimmed == "apps:" {
			continue
		}

		// New app entry "  - name: ..." or "  - name:" on its own line.
		if strings.HasPrefix(line, "  - ") {
			// First field of a new app block.
			rest := strings.TrimPrefix(line, "  - ")
			cfg.Apps = append(cfg.Apps, AppConfig{})
			current = &cfg.Apps[len(cfg.Apps)-1]
			if err := applyField(current, rest); err != nil {
				return nil, err
			}
			continue
		}

		// Continuation of current app block (4+ spaces indent or branch list items).
		if current != nil {
			// Branch list item "      - main"
			if strings.HasPrefix(trimmed, "- ") {
				branch := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				current.Branches = append(current.Branches, branch)
				continue
			}

			// Scalar field "    key: value"
			if strings.HasPrefix(line, "    ") {
				if err := applyField(current, strings.TrimSpace(line)); err != nil {
					return nil, err
				}
				continue
			}
		}
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("multiapp: reading %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyField parses a "key: value" line and sets the appropriate field on app.
func applyField(app *AppConfig, line string) error {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return nil // skip lines without ":"
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	value = stripQuotes(value)

	switch key {
	case "name":
		app.Name = value
	case "repo":
		app.Repo = value
	case "base_dir":
		app.BaseDir = value
	case "branches":
		// "branches:" with no value — items follow on subsequent lines.
		// Nothing to do here; handled in ParseFile via "- item" lines.
	}
	return nil
}

// validate checks that every app has the required fields.
func (c *Config) validate() error {
	seen := map[string]bool{}
	for i, app := range c.Apps {
		if app.Name == "" {
			return fmt.Errorf("multiapp: app[%d] is missing 'name'", i)
		}
		if app.Repo == "" {
			return fmt.Errorf("multiapp: app %q is missing 'repo'", app.Name)
		}
		if app.BaseDir == "" {
			return fmt.Errorf("multiapp: app %q is missing 'base_dir'", app.Name)
		}
		if seen[app.BaseDir] {
			return fmt.Errorf("multiapp: duplicate base_dir %q (each app must have a unique directory)", app.BaseDir)
		}
		seen[app.BaseDir] = true
	}
	return nil
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// BranchAllowed reports whether branch is permitted to deploy this app.
// Uses the same semantics as manifest.BranchAllowed.
func (a *AppConfig) BranchAllowed(branch string) bool {
	if len(a.Branches) == 0 || branch == "" {
		return true
	}
	for _, pattern := range a.Branches {
		if matchBranch(pattern, branch) {
			return true
		}
	}
	return false
}

func matchBranch(pattern, branch string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(branch, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == branch
}
