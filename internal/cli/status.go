package cli

import (
	"fmt"

	"github.com/Reeteshrajesh/runway/internal/release"
)

func runStatus(args []string) error {
	fs := newFlagSet("status")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dir := baseDir()
	mgr := release.NewManager(dir)
	hist := release.NewHistory(dir)

	active, err := mgr.ActiveCommit()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	h, err := hist.Load()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	fmt.Println("─────────────────────────────────────")
	fmt.Printf(" runway status\n")
	fmt.Println("─────────────────────────────────────")

	if active == "" {
		fmt.Println(" active:   (none)")
	} else {
		fmt.Printf(" active:   %s\n", active)
	}

	fmt.Println()
	fmt.Println(" recent deployments:")
	fmt.Printf(" %-14s  %-20s  %s\n", "commit", "time", "status")
	fmt.Println(" " + repeat("─", 54))

	limit := 5
	for i, d := range h.Deployments {
		if i >= limit {
			break
		}
		fmt.Printf(" %-14s  %-20s  %s\n",
			shortSHA(d.Commit),
			d.Time.Local().Format("2006-01-02 15:04:05"),
			d.Status,
		)
	}

	if len(h.Deployments) == 0 {
		fmt.Println(" (no deployments yet)")
	}

	fmt.Println("─────────────────────────────────────")
	return nil
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
