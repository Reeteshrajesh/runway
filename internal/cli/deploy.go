package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Reeteshrajesh/runway/internal/color"
	"github.com/Reeteshrajesh/runway/internal/engine"
)

func runDeploy(args []string) error {
	fs := newFlagSet("deploy")
	dryRun := fs.Bool("dry-run", false, "Validate and print what would run without executing")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway deploy [--dry-run] <commit>")
		fmt.Fprintln(os.Stderr, "\nDeploy a specific git commit.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
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
		return exitErr(ExitManifestError, fmt.Errorf("GITOPS_REPO environment variable is not set"))
	}

	if *dryRun {
		return runDryRun(commit, repo)
	}

	cfg := engine.Config{
		BaseDir:   baseDir(),
		RepoURL:   repo,
		Commit:    commit,
		GitToken:  gitToken(),
		Triggered: "cli",
	}

	color.Infof(os.Stdout, "deploying %s", color.Bold(shortSHA(commit)))
	result := engine.Deploy(cfg)

	if !result.Success {
		if result.AutoRolledBack {
			color.Warnf(os.Stdout, "start failed — auto-rolled back to %s", shortSHA(result.RolledBackTo))
		}
		color.Errorf(os.Stderr, "deploy failed: %v", result.Err)
		return exitErr(deployExitCode(result), result.Err)
	}

	color.Successf(os.Stdout, "deployed %s in %.1fs", shortSHA(commit), result.EndedAt.Sub(result.StartedAt).Seconds())
	return nil
}

// deployExitCode maps a DeployResult to the appropriate exit code.
func deployExitCode(r engine.DeployResult) int {
	if r.Err == nil {
		return ExitOK
	}
	msg := r.Err.Error()
	switch {
	case strings.Contains(msg, "already in progress"):
		return ExitLockHeld
	case strings.Contains(msg, "clone:") || strings.Contains(msg, "git "):
		return ExitGitError
	case strings.Contains(msg, "manifest:"):
		return ExitManifestError
	case strings.Contains(msg, "start failed"):
		return ExitStartFailed
	case strings.Contains(msg, "setup:") || strings.Contains(msg, "build:"):
		return ExitBuildFailed
	default:
		return ExitGeneralError
	}
}
