package manifest

import (
	"fmt"
	"strings"
)

// suspiciousPatterns are shell constructs that are unusual in build commands.
// runway runs all commands via /bin/sh -c, so these are valid shell — but
// operators should be aware if they appear unexpectedly in manifest.yml.
var suspiciousPatterns = []string{
	"$(",    // command substitution $(...)
	"`",     // backtick command substitution
	"eval ", // eval — arbitrary code execution
	"exec ", // exec — replaces process
	"curl ", // network fetch — could exfiltrate or download
	"wget ", // network fetch
	"nc ",   // netcat — often used for shells
	"bash -c",
	"sh -c",
}

// AuditWarning describes a potential security concern in a manifest command.
type AuditWarning struct {
	Field   string // "setup", "build", or "start"
	Command string
	Pattern string
}

func (w AuditWarning) String() string {
	return fmt.Sprintf("manifest audit: %s command %q contains %q — verify this is intentional", w.Field, w.Command, w.Pattern)
}

// Audit inspects all commands in the manifest for suspicious shell patterns.
// Returns a (possibly empty) list of warnings. Audit never returns errors —
// warnings are advisory only and never abort a deploy.
func (m *Manifest) Audit() []AuditWarning {
	var warnings []AuditWarning
	for field, cmds := range map[string][]string{
		"setup":       m.Setup,
		"build":       m.Build,
		"start":       m.Start,
		"pre_deploy":  m.PreDeploy,
		"post_deploy": m.PostDeploy,
	} {
		for _, cmd := range cmds {
			for _, pat := range suspiciousPatterns {
				if strings.Contains(cmd, pat) {
					warnings = append(warnings, AuditWarning{
						Field:   field,
						Command: cmd,
						Pattern: pat,
					})
					break // one warning per command is enough
				}
			}
		}
	}
	return warnings
}
