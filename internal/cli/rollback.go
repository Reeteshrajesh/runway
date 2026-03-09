package cli

import (
	"fmt"
	"os"

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

	fmt.Printf("rolling back to commit %s ...\n", commit)
	result := engine.Rollback(cfg)

	if !result.Success {
		return fmt.Errorf("rollback failed: %v", result.Err)
	}

	fmt.Printf("rolled back to %s in %.1fs\n", commit, result.EndedAt.Sub(result.StartedAt).Seconds())
	return nil
}
