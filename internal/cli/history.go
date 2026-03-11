package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Reeteshrajesh/runway/internal/release"
)

func runHistory(args []string) error {
	fs := newFlagSet("history")
	limit := fs.Int("limit", 0, "Maximum number of entries to show (0 = all)")
	statusFilter := fs.String("status", "", "Filter by status (running, success, failed, previous, rolled_back)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway history [flags]")
		fmt.Fprintln(os.Stderr, "\nShow full deployment history.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	dir := baseDir()
	hist := release.NewHistory(dir)
	h, err := hist.Load()
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}

	deployments := h.Deployments
	if *statusFilter != "" {
		filtered := deployments[:0]
		for _, d := range deployments {
			if string(d.Status) == *statusFilter {
				filtered = append(filtered, d)
			}
		}
		deployments = filtered
	}

	if *limit > 0 && len(deployments) > *limit {
		deployments = deployments[:*limit]
	}

	sep := strings.Repeat("─", 72)
	fmt.Printf(" %-14s  %-20s  %-14s  %s\n", "COMMIT", "TIME", "STATUS", "BY")
	fmt.Printf(" %s\n", sep)

	for _, d := range deployments {
		fmt.Printf(" %-14s  %-20s  %-14s  %s\n",
			shortSHA(d.Commit),
			d.Time.Local().Format("2006-01-02 15:04:05"),
			colorStatus(string(d.Status)),
			d.Triggered,
		)
	}

	if len(deployments) == 0 {
		if *statusFilter != "" {
			fmt.Printf(" (no deployments with status %q)\n", *statusFilter)
		} else {
			fmt.Println(" (no deployments yet)")
		}
	}

	return nil
}
