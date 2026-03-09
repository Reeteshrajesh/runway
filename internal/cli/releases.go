package cli

import (
	"fmt"

	"github.com/Reeteshrajesh/runway/internal/release"
)

func runReleases(args []string) error {
	fs := newFlagSet("releases")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mgr := release.NewManager(baseDir())

	active, err := mgr.ActiveCommit()
	if err != nil {
		return fmt.Errorf("releases: %w", err)
	}

	releases, err := mgr.ListReleases()
	if err != nil {
		return fmt.Errorf("releases: %w", err)
	}

	if len(releases) == 0 {
		fmt.Println("no releases found")
		return nil
	}

	fmt.Printf("%-14s  %s\n", "commit", "active")
	fmt.Println(repeat("─", 22))

	// Print newest first.
	for i := len(releases) - 1; i >= 0; i-- {
		commit := releases[i]
		marker := ""
		if commit == active {
			marker = "  ← current"
		}
		fmt.Printf("%-14s%s\n", shortSHA(commit), marker)
	}

	return nil
}
