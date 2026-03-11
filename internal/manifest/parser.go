// Package manifest parses the runway file (manifest.yml).
//
// It supports a strict subset of YAML sufficient for the manifest schema:
//   - Top-level scalar string:  key: value
//   - Top-level string array:   key:\n  - item1\n  - item2
//
// Full YAML (anchors, multiline strings, flow mappings, etc.) is NOT supported.
package manifest

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultTimeoutSeconds is the deploy timeout when not specified in manifest.
const DefaultTimeoutSeconds = 600 // 10 minutes

// NotifyEmail holds SMTP email notification settings (all optional).
type NotifyEmail struct {
	To       string // recipient address
	From     string // sender address
	SMTPHost string // e.g. smtp.gmail.com
	SMTPPort string // e.g. 587
}

// HealthCheck holds optional zero-downtime health check configuration.
// When configured, runway polls the URL after start commands and only
// flips the symlink once it returns HTTP 200.
type HealthCheck struct {
	// URL is the HTTP/HTTPS endpoint to poll.
	URL string

	// Interval is how often to poll in seconds (default 2).
	Interval int

	// Retries is the max number of attempts before giving up (default 10).
	Retries int
}

// Manifest holds the parsed contents of a manifest.yml file.
type Manifest struct {
	// App is the application name (required).
	App string

	// EnvFile is the path to the .env file on the server (optional).
	EnvFile string

	// Setup contains commands to install dependencies (optional).
	Setup []string

	// Build contains commands to build the project (optional).
	Build []string

	// Start contains commands to start the service (required).
	Start []string

	// TimeoutSeconds is the maximum allowed deploy duration in seconds.
	// Defaults to DefaultTimeoutSeconds (600s) if not set.
	TimeoutSeconds int

	// PreDeploy contains commands run before setup/build/start.
	// Runs in the release directory. Failure aborts the deploy.
	PreDeploy []string

	// PostDeploy contains commands run after a successful start.
	// Runs in the release directory. Failure is logged but does not revert the deploy.
	PostDeploy []string

	// HealthCheck holds optional zero-downtime health check settings.
	HealthCheck HealthCheck

	// Branches is an optional list of branch name patterns allowed to trigger a deploy.
	// Patterns support a single trailing wildcard (*). Empty = all branches allowed.
	// Examples: ["main"], ["main", "release/*"]
	Branches []string

	// Notify holds optional notification settings.
	Notify NotifyEmail
}

// Validate checks that all required fields are present and sets defaults.
func (m *Manifest) Validate() error {
	if strings.TrimSpace(m.App) == "" {
		return fmt.Errorf("manifest: 'app' field is required")
	}
	if len(m.Start) == 0 {
		return fmt.Errorf("manifest: 'start' field is required and must have at least one command")
	}
	if m.TimeoutSeconds <= 0 {
		m.TimeoutSeconds = DefaultTimeoutSeconds
	}
	return nil
}

// ParseFile reads and parses the manifest file at the given path.
func ParseFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: cannot open %s: %w", path, err)
	}
	defer f.Close()

	m := &Manifest{}
	scanner := bufio.NewScanner(f)

	// currentKey tracks which array field we are currently appending items to.
	// currentSection tracks the top-level section for nested scalar blocks
	// (e.g. "notify" → "notify.email").
	currentKey := ""
	currentSection := "" // "notify.email" when parsing notify: / email: block

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimRight(raw, " \t")

		// Skip blank lines and comments.
		// Do NOT reset currentKey — a comment or blank line inside a list block
		// does not close it. Only a non-list key line closes the block.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Array item: lines starting with "  - " or "- " while inside an array block.
		if currentKey != "" && isListItem(line) {
			value := parseListItem(line)
			switch currentKey {
			case "setup":
				m.Setup = append(m.Setup, value)
			case "build":
				m.Build = append(m.Build, value)
			case "start":
				m.Start = append(m.Start, value)
			case "pre_deploy":
				m.PreDeploy = append(m.PreDeploy, value)
			case "post_deploy":
				m.PostDeploy = append(m.PostDeploy, value)
			case "branches":
				m.Branches = append(m.Branches, value)
			}
			continue
		}

		// Any non-list line closes the current array block.
		currentKey = ""

		// Indented lines (2+ spaces) inside a nested section block.
		if currentSection != "" && isIndented(line) {
			key, value, err := parseLine(strings.TrimSpace(line), lineNum)
			if err == nil {
				switch {
				case currentSection == "notify" && key == "email" && value == "":
					// "  email:" — advance into the email sub-section.
					currentSection = "notify.email"
				case currentSection == "notify.email":
					switch key {
					case "to":
						m.Notify.To = value
					case "from":
						m.Notify.From = value
					case "smtp_host":
						m.Notify.SMTPHost = value
					case "smtp_port":
						m.Notify.SMTPPort = value
					}
				case currentSection == "health_check":
					switch key {
					case "url":
						m.HealthCheck.URL = value
					case "interval":
						if n, err2 := strconv.Atoi(value); err2 == nil && n > 0 {
							m.HealthCheck.Interval = n
						}
					case "retries":
						if n, err2 := strconv.Atoi(value); err2 == nil && n > 0 {
							m.HealthCheck.Retries = n
						}
					}
				}
			}
			continue
		}
		// Non-indented line closes any active section.
		currentSection = ""

		// Must be a key: value or key: line.
		key, value, err := parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}

		if value == "" {
			// key with no inline value — start of an array or section block.
			switch key {
			case "setup", "build", "start", "pre_deploy", "post_deploy", "branches":
				currentKey = key
			case "notify":
				currentSection = "notify"
			case "health_check":
				currentSection = "health_check"
			case "email":
				if currentSection == "notify" {
					currentSection = "notify.email"
				}
			default:
				// Unknown block key — skip silently.
			}
			continue
		}

		// Scalar assignment.
		switch key {
		case "app":
			m.App = value
		case "env_file":
			m.EnvFile = value
		case "timeout":
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("manifest: line %d: 'timeout' must be a positive integer (seconds), got: %q", lineNum, value)
			}
			m.TimeoutSeconds = n
		case "setup":
			// Inline scalar under setup — treat as single command.
			m.Setup = append(m.Setup, value)
		case "build":
			m.Build = append(m.Build, value)
		case "start":
			m.Start = append(m.Start, value)
		case "pre_deploy":
			m.PreDeploy = append(m.PreDeploy, value)
		case "post_deploy":
			m.PostDeploy = append(m.PostDeploy, value)
			// Unknown keys are ignored — forward-compatible.
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("manifest: reading %s: %w", path, err)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return m, nil
}

// parseLine splits a YAML scalar line "key: value" into its key and value.
// The value may be empty if the line is "key:" (start of an array block).
func parseLine(line string, lineNum int) (key, value string, err error) {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("manifest: line %d: expected 'key: value', got: %q", lineNum, line)
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	value = stripQuotes(value)
	return key, value, nil
}

// isIndented returns true if the line starts with at least two spaces or a tab,
// indicating it belongs to a nested block (e.g. under notify: / email:).
func isIndented(line string) bool {
	return strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")
}

// isListItem returns true if the line looks like a YAML list item ("  - ...").
func isListItem(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "- ")
}

// parseListItem extracts the value from a YAML list item line ("  - value").
func parseListItem(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	value := strings.TrimPrefix(trimmed, "- ")
	return stripQuotes(strings.TrimSpace(value))
}

// stripQuotes removes surrounding single or double quotes from a string value.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
