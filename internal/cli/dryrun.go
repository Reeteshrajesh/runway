package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Reeteshrajesh/runway/internal/color"
	"github.com/Reeteshrajesh/runway/internal/manifest"
)

// runDryRun validates the configuration and prints what would be executed
// without making any changes to the server.
func runDryRun(commit, repo string) error {
	dir := baseDir()
	manifestPath := filepath.Join(dir, "manifest.yml")

	fmt.Printf("\n %s\n\n", color.Bold("DRY RUN — no changes will be made"))

	allOK := true
	check := func(ok bool, format string, a ...any) {
		if ok {
			color.Successf(os.Stdout, format, a...)
		} else {
			color.Errorf(os.Stdout, format, a...)
			allOK = false
		}
	}

	// ── 1. git binary ─────────────────────────────────────────────────────────
	_, gitErr := exec.LookPath("git")
	check(gitErr == nil, "git is installed")

	// ── 2. GITOPS_REPO is set ─────────────────────────────────────────────────
	check(repo != "", "GITOPS_REPO is set (%s)", repo)

	// ── 3. GITOPS_DIR exists ──────────────────────────────────────────────────
	_, dirErr := os.Stat(dir)
	check(dirErr == nil, "GITOPS_DIR exists (%s)", dir)

	// ── 4. releases/ directory ────────────────────────────────────────────────
	releasesDir := filepath.Join(dir, "releases")
	_, relErr := os.Stat(releasesDir)
	check(relErr == nil, "releases/ directory exists (%s)", releasesDir)

	// ── 5. manifest.yml ───────────────────────────────────────────────────────
	mf, mfErr := manifest.ParseFile(manifestPath)
	check(mfErr == nil, "manifest.yml valid (app: %s)", func() string {
		if mfErr != nil {
			return mfErr.Error()
		}
		return mf.App
	}())

	// ── 6. .env file (if configured) ─────────────────────────────────────────
	if mf != nil && mf.EnvFile != "" {
		_, envErr := os.Stat(mf.EnvFile)
		check(envErr == nil, ".env file readable (%s)", mf.EnvFile)
	}

	fmt.Println()

	if mf != nil {
		fmt.Printf(" %s\n", color.Bold("Would run:"))
		printCommandList("setup", mf.Setup)
		printCommandList("build", mf.Build)
		printCommandList("start", mf.Start)
		fmt.Printf("\n %s timeout: %ds\n", color.Arrow(), mf.TimeoutSeconds)
	}

	fmt.Println()

	if !allOK {
		color.Errorf(os.Stdout, "fix the issues above before deploying")
		return fmt.Errorf("dry-run found problems")
	}

	color.Infof(os.Stdout, "run without --dry-run to deploy commit %s", shortSHA(commit))
	return nil
}

func printCommandList(label string, cmds []string) {
	if len(cmds) == 0 {
		return
	}
	fmt.Printf("   %-8s", label+":")
	for i, c := range cmds {
		if i == 0 {
			fmt.Printf(" %s\n", c)
		} else {
			fmt.Printf("            %s\n", c)
		}
	}
}
