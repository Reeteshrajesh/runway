package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Reeteshrajesh/runway/internal/color"
	"github.com/Reeteshrajesh/runway/internal/engine"
)

func runRollback(args []string) error {
	fs := newFlagSet("rollback")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway rollback <commit>")
		fmt.Fprintln(os.Stderr, "\nRoll back to a previously deployed commit (no rebuild required).")
		fmt.Fprintln(os.Stderr, "\nRun 'runway releases' to see available commits.")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := requireArg(fs.Args(), 1, "runway rollback <commit>"); err != nil {
		fs.Usage()
		return err
	}

	commit := fs.Arg(0)

	cfg := engine.Config{
		BaseDir:   baseDir(),
		Commit:    commit,
		Triggered: "cli",
	}

	color.Infof(os.Stdout, "rolling back to %s", color.Bold(shortSHA(commit)))
	result := engine.Rollback(cfg)

	if !result.Success {
		color.Errorf(os.Stderr, "rollback failed: %v", result.Err)
		return exitErr(rollbackExitCode(result), result.Err)
	}

	color.Successf(os.Stdout, "rolled back to %s in %.1fs", shortSHA(commit), result.EndedAt.Sub(result.StartedAt).Seconds())
	return nil
}

func rollbackExitCode(r engine.DeployResult) int {
	if r.Err == nil {
		return ExitOK
	}
	msg := r.Err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return ExitNotFound
	case strings.Contains(msg, "already in progress"):
		return ExitLockHeld
	default:
		return ExitGeneralError
	}
}
