// Package notify sends deploy outcome notifications via email (net/smtp).
// SMTP password is read from the environment as RUNWAY_SMTP_PASSWORD —
// it must never appear in manifest.yml.
package notify

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/Reeteshrajesh/runway/internal/manifest"
)

// DeployEvent describes what happened during a deploy, used to compose the email.
type DeployEvent struct {
	App          string
	Commit       string
	Triggered    string // "cli" | "webhook"
	Duration     time.Duration
	Err          error // nil on success
	RolledBack   bool
	RolledBackTo string
	// LastLogLines are the final lines of deploy.log, included in failure emails.
	LastLogLines []string
}

// SendDeployEmail sends a notification email for the given deploy event.
// Returns nil (silently) if notification is not configured in the manifest.
// SMTP password is read from RUNWAY_SMTP_PASSWORD env var.
func SendDeployEmail(cfg manifest.NotifyEmail, ev DeployEvent) error {
	if cfg.To == "" || cfg.SMTPHost == "" {
		return nil // notifications not configured
	}

	from := cfg.From
	if from == "" {
		from = "runway@localhost"
	}
	port := cfg.SMTPPort
	if port == "" {
		port = "587"
	}

	subject, body := composeEmail(cfg, ev)
	password := os.Getenv("RUNWAY_SMTP_PASSWORD")

	addr := cfg.SMTPHost + ":" + port

	var auth smtp.Auth
	if password != "" {
		auth = smtp.PlainAuth("", from, password, cfg.SMTPHost)
	}

	msg := buildMessage(from, cfg.To, subject, body)
	return smtp.SendMail(addr, auth, from, []string{cfg.To}, []byte(msg))
}

func composeEmail(cfg manifest.NotifyEmail, ev DeployEvent) (subject, body string) {
	_ = cfg
	shortCommit := ev.Commit
	if len(shortCommit) > 12 {
		shortCommit = shortCommit[:12]
	}
	durStr := fmt.Sprintf("%.1fs", ev.Duration.Seconds())

	switch {
	case ev.Err == nil:
		subject = fmt.Sprintf("✓ runway: deployed %s to %s", shortCommit, ev.App)
		body = fmt.Sprintf(
			"Deploy succeeded.\n\nApp:       %s\nCommit:    %s\nTriggered: %s\nDuration:  %s\n",
			ev.App, ev.Commit, ev.Triggered, durStr,
		)
	case ev.RolledBack:
		rolledTo := ev.RolledBackTo
		if len(rolledTo) > 12 {
			rolledTo = rolledTo[:12]
		}
		subject = fmt.Sprintf("⚠ runway: auto-rolled back %s to %s", ev.App, rolledTo)
		body = fmt.Sprintf(
			"Deploy failed and was automatically rolled back.\n\nApp:            %s\nFailed commit:  %s\nRolled back to: %s\nTriggered:      %s\nDuration:       %s\nError:          %v\n",
			ev.App, ev.Commit, ev.RolledBackTo, ev.Triggered, durStr, ev.Err,
		)
	default:
		subject = fmt.Sprintf("✗ runway: deploy failed on %s", ev.App)
		body = fmt.Sprintf(
			"Deploy failed.\n\nApp:       %s\nCommit:    %s\nTriggered: %s\nDuration:  %s\nError:     %v\n",
			ev.App, ev.Commit, ev.Triggered, durStr, ev.Err,
		)
	}

	if len(ev.LastLogLines) > 0 && ev.Err != nil {
		body += "\n--- last deploy.log lines ---\n" + strings.Join(ev.LastLogLines, "\n") + "\n"
	}
	return subject, body
}

func buildMessage(from, to, subject, body string) string {
	return "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		body
}

// ComposeEmailForTest exposes composeEmail for unit testing.
func ComposeEmailForTest(cfg manifest.NotifyEmail, ev DeployEvent) (subject, body string) {
	return composeEmail(cfg, ev)
}
