// Package envloader reads KEY=VALUE pairs from a .env file and returns
// them as a slice suitable for use with os/exec Cmd.Env.
//
// Rules:
//   - Blank lines are ignored.
//   - Lines starting with # are treated as comments and ignored.
//   - Values may be wrapped in single or double quotes (quotes are stripped).
//   - No shell variable expansion is performed.
//   - Duplicate keys are allowed; the last value wins.
package envloader

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads the .env file at path and returns a slice of "KEY=VALUE" strings.
// Returns an empty slice (not an error) if path is empty.
func Load(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("envloader: cannot open %s: %w", path, err)
	}
	defer f.Close()

	var pairs []string
	seen := make(map[string]int) // key -> index in pairs for dedup
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, err := splitPair(line, lineNum)
		if err != nil {
			return nil, err
		}

		entry := key + "=" + value
		if idx, exists := seen[key]; exists {
			// Overwrite previous value for this key.
			pairs[idx] = entry
		} else {
			seen[key] = len(pairs)
			pairs = append(pairs, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("envloader: reading %s: %w", path, err)
	}

	return pairs, nil
}

// Merge combines the base environment (typically os.Environ()) with the
// loaded .env pairs. .env values take precedence (last value wins in exec.Cmd.Env).
func Merge(base, loaded []string) []string {
	merged := make([]string, len(base)+len(loaded))
	copy(merged, base)
	copy(merged[len(base):], loaded)
	return merged
}

// splitPair splits a "KEY=VALUE" line into its key and value components.
func splitPair(line string, lineNum int) (key, value string, err error) {
	idx := strings.Index(line, "=")
	if idx == -1 {
		return "", "", fmt.Errorf("envloader: line %d: expected KEY=VALUE, got: %q", lineNum, line)
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	value = stripQuotes(value)

	if key == "" {
		return "", "", fmt.Errorf("envloader: line %d: empty key", lineNum)
	}

	return key, value, nil
}

// stripQuotes removes surrounding single or double quotes from a value.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
