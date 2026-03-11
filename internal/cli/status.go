package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Reeteshrajesh/runway/internal/color"
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

	fmt.Println(color.Bold("─────────────────────────────────────"))
	fmt.Printf(" %s\n", color.Bold("runway status"))
	fmt.Println(color.Bold("─────────────────────────────────────"))

	if active == "" {
		fmt.Println(" active:   (none)")
	} else {
		fmt.Printf(" active:   %s\n", color.Cyan(active))
	}

	fmt.Println()
	fmt.Println(" recent deployments:")
	fmt.Printf(" %-14s  %-20s  %s\n", "commit", "time", "status")
	fmt.Println(" " + strings.Repeat("─", 54))

	limit := 5
	for i, d := range h.Deployments {
		if i >= limit {
			break
		}
		statusStr := colorStatus(string(d.Status))
		fmt.Printf(" %-14s  %-20s  %s\n",
			shortSHA(d.Commit),
			d.Time.Local().Format("2006-01-02 15:04:05"),
			statusStr,
		)
	}

	if len(h.Deployments) == 0 {
		fmt.Println(" (no deployments yet)")
	}

	fmt.Println(color.Bold("─────────────────────────────────────"))
	return nil
}

// colorStatus applies a color to a deployment status string.
func colorStatus(s string) string {
	switch release.DeployStatus(s) {
	case release.StatusSuccess:
		return color.Green(s)
	case release.StatusFailed:
		return color.Red(s)
	case release.StatusRunning:
		return color.Cyan(s)
	case release.StatusRolledBack:
		return color.Yellow(s)
	default:
		return s
	}
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func printTable(w *os.File, header [4]string, rows [][4]string) {
	sep := strings.Repeat("─", 72)
	fmt.Fprintf(w, " %-14s  %-20s  %-14s  %s\n", header[0], header[1], header[2], header[3])
	fmt.Fprintf(w, " %s\n", sep)
	for _, r := range rows {
		fmt.Fprintf(w, " %-14s  %-20s  %-14s  %s\n", r[0], r[1], r[2], r[3])
	}
}
