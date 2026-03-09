package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runLog(args []string) error {
	fs := newFlagSet("log")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway log <commit>")
		fmt.Fprintln(os.Stderr, "\nPrint the deploy log for a specific commit.")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := requireArg(fs.Args(), 1, "runway log <commit>"); err != nil {
		fs.Usage()
		return err
	}

	commit := fs.Arg(0)
	logPath := filepath.Join(baseDir(), "releases", commit, "deploy.log")

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no deploy log found for commit %q\n  (looked at: %s)", commit, logPath)
		}
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(os.Stdout, f)
	return err
}
