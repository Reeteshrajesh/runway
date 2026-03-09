package cli

import (
	"fmt"
	"os"

	"github.com/Reeteshrajesh/runway/internal/engine"
	"github.com/Reeteshrajesh/runway/internal/webhook"
)

func runListen(args []string) error {
	fs := newFlagSet("listen")
	port := fs.Int("port", 9000, "TCP port to listen on")
	secret := fs.String("secret", "", "Webhook HMAC signing secret (required)")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway listen [flags]")
		fmt.Fprintln(os.Stderr, "\nStart the webhook listener HTTP server.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nEnvironment variables:")
		fmt.Fprintln(os.Stderr, "  GITOPS_REPO         Git repository URL (required)")
		fmt.Fprintln(os.Stderr, "  GITOPS_DIR          Working directory (default: /opt/runway)")
		fmt.Fprintln(os.Stderr, "  GITOPS_GIT_TOKEN    Git HTTPS auth token (optional)")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  runway listen --port 9000 --secret $WEBHOOK_SECRET")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Allow secret from environment variable as an alternative to the flag.
	if *secret == "" {
		*secret = os.Getenv("GITOPS_WEBHOOK_SECRET")
	}

	if *secret == "" {
		fs.Usage()
		return fmt.Errorf("--secret flag or GITOPS_WEBHOOK_SECRET env var is required")
	}

	repo := repoURL()
	if repo == "" {
		return fmt.Errorf("GITOPS_REPO environment variable is not set")
	}

	cfg := webhook.Config{
		Port:   *port,
		Secret: *secret,
		DeployConfig: engine.Config{
			BaseDir:  baseDir(),
			RepoURL:  repo,
			GitToken: gitToken(),
		},
	}

	srv := webhook.New(cfg)
	return srv.Start()
}
