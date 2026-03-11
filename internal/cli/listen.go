package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Reeteshrajesh/runway/internal/engine"
	"github.com/Reeteshrajesh/runway/internal/logger"
	"github.com/Reeteshrajesh/runway/internal/webhook"
)

func runListen(args []string) error {
	fs := newFlagSet("listen")
	port := fs.Int("port", 9000, "TCP port to listen on")
	secret := fs.String("secret", "", "Webhook HMAC signing secret (required)")
	logFormat := fs.String("log-format", "text", "Event log format: text or json")
	rateLimit := fs.Int("webhook-rate-limit", 5, "Max webhook requests per minute (0 = unlimited)")

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
		fmt.Fprintln(os.Stderr, "  runway listen --port 9000 --secret $WEBHOOK_SECRET --log-format json")
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

	var fmt_ logger.Format
	switch *logFormat {
	case "json":
		fmt_ = logger.FormatJSON
	default:
		fmt_ = logger.FormatText
	}

	cfg := webhook.Config{
		Port:      *port,
		Secret:    *secret,
		EventLog:  logger.NewEventLogger(os.Stderr, fmt_),
		RateLimit: *rateLimit,
		DeployConfig: engine.Config{
			BaseDir:  baseDir(),
			RepoURL:  repo,
			GitToken: gitToken(),
		},
	}

	srv := webhook.New(cfg)

	// ── Graceful shutdown on SIGINT / SIGTERM ─────────────────────────────────
	// signal.NotifyContext cancels ctx when either signal is received.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the HTTP server in a goroutine; capture any startup error.
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Start()
	}()

	// Block until a signal fires or the server exits on its own.
	select {
	case err := <-serveErr:
		// Server stopped before a signal — surface the error unless it is the
		// expected ErrServerClosed returned after a normal Shutdown call.
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil

	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "\nshutting down — waiting for in-flight deploy to finish…")
	}

	// Give open HTTP connections up to 10 s to drain, then wait for deploys.
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	fmt.Fprintln(os.Stderr, "runway stopped cleanly")
	return nil
}
