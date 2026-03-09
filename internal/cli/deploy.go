package cli

import (
	"fmt"
	"os"

	"github.com/Reeteshrajesh/runway/internal/engine"
)

func runDeploy(args []string) error {
	fs := newFlagSet("deploy")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway deploy <commit>")
		fmt.Fprintln(os.Stderr, "\nDeploy a specific git commit.")
		fmt.Fprintln(os.Stderr, "\nEnvironment variables:")
		fmt.Fprintln(os.Stderr, "  GITOPS_REPO         Git repository URL (required)")
		fmt.Fprintln(os.Stderr, "  GITOPS_DIR          Working directory (default: /opt/runway)")
		fmt.Fprintln(os.Stderr, "  GITOPS_GIT_TOKEN    Git HTTPS auth token (optional)")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := requireArg(fs.Args(), 1, "runway deploy <commit>"); err != nil {
		fs.Usage()
		return err
	}

	commit := fs.Arg(0)
	repo := repoURL()
	if repo == "" {
		return fmt.Errorf("GITOPS_REPO environment variable is not set")
	}

	cfg := engine.Config{
		BaseDir:   baseDir(),
		RepoURL:   repo,
		Commit:    commit,
		GitToken:  gitToken(),
		Triggered: "cli",
	}

	fmt.Printf("deploying commit %s ...\n", commit)
	result := engine.Deploy(cfg)

	if !result.Success {
		return fmt.Errorf("deploy failed: %v", result.Err)
	}

	fmt.Printf("deployed successfully in %.1fs\n", result.EndedAt.Sub(result.StartedAt).Seconds())
	return nil
}
