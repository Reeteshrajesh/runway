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
	"strings"
)

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
}

// Validate checks that all required fields are present.
func (m *Manifest) Validate() error {
	if strings.TrimSpace(m.App) == "" {
		return fmt.Errorf("manifest: 'app' field is required")
	}
	if len(m.Start) == 0 {
		return fmt.Errorf("manifest: 'start' field is required and must have at least one command")
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
	// Empty string means we are not inside an array block.
	currentKey := ""

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
			}
			continue
		}

		// Any non-list line closes the current array block.
		currentKey = ""

		// Must be a key: value or key: line.
		key, value, err := parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}

		if value == "" {
			// key with no inline value — start of an array block.
			switch key {
			case "setup", "build", "start":
				currentKey = key
			default:
				// Unknown array key — skip silently.
			}
			continue
		}

		// Scalar assignment.
		switch key {
		case "app":
			m.App = value
		case "env_file":
			m.EnvFile = value
		case "setup":
			// Inline scalar under setup — treat as single command.
			m.Setup = append(m.Setup, value)
		case "build":
			m.Build = append(m.Build, value)
		case "start":
			m.Start = append(m.Start, value)
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
