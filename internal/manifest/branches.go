package manifest

import "strings"

// BranchAllowed reports whether branch is permitted to trigger a deploy.
//
// Rules:
//   - If m.Branches is empty, all branches are allowed.
//   - Each pattern is matched case-sensitively.
//   - A trailing "*" is the only supported wildcard; it matches any suffix.
//     e.g. "release/*" matches "release/1.2" and "release/hotfix".
//   - Exact matches are also supported: "main" only matches "main".
func (m *Manifest) BranchAllowed(branch string) bool {
	if len(m.Branches) == 0 {
		return true // no rules configured → allow all
	}
	if branch == "" {
		// No branch info (e.g. CLI deploy) → allow; rules only apply to webhook.
		return true
	}
	for _, pattern := range m.Branches {
		if matchBranch(pattern, branch) {
			return true
		}
	}
	return false
}

// matchBranch returns true if branch matches pattern.
// Supports a single trailing wildcard "*".
func matchBranch(pattern, branch string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(branch, prefix)
	}
	return pattern == branch
}
