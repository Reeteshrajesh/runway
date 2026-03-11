package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Reeteshrajesh/runway/internal/color"
	"github.com/Reeteshrajesh/runway/internal/engine"
	"github.com/Reeteshrajesh/runway/internal/manifest"
)

func runDoctor(args []string) error {
	fs := newFlagSet("doctor")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runway doctor")
		fmt.Fprintln(os.Stderr, "\nCheck runway setup and diagnose configuration problems.")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	dir := baseDir()
	manifestPath := filepath.Join(dir, "manifest.yml")
	issues := 0

	fmt.Printf("\n %s\n\n", color.Bold("Checking runway setup..."))

	check := func(ok bool, msg, fix string) {
		if ok {
			color.Successf(os.Stdout, "%s", msg)
		} else {
			color.Errorf(os.Stdout, "%s", msg)
			if fix != "" {
				fmt.Printf("      %s Fix: %s\n", color.Arrow(), fix)
			}
			issues++
		}
	}

	// ── git binary ────────────────────────────────────────────────────────────
	gitPath, gitErr := exec.LookPath("git")
	if gitErr == nil {
		out, _ := exec.Command(gitPath, "version").Output()
		check(true, fmt.Sprintf("git is installed (%s)", trimNL(string(out))), "")
	} else {
		check(false, "git is NOT installed", "install git: https://git-scm.com/downloads")
	}

	// ── GITOPS_REPO ───────────────────────────────────────────────────────────
	repo := repoURL()
	check(repo != "", fmt.Sprintf("GITOPS_REPO is set (%s)", repo),
		"export GITOPS_REPO=<your-git-url>")

	// ── GITOPS_DIR exists and is writable ─────────────────────────────────────
	_, dirErr := os.Stat(dir)
	check(dirErr == nil, fmt.Sprintf("GITOPS_DIR exists (%s)", dir),
		fmt.Sprintf("mkdir -p %s", dir))

	if dirErr == nil {
		tmpFile := filepath.Join(dir, ".runway-write-check")
		writeErr := os.WriteFile(tmpFile, []byte(""), 0600)
		if writeErr == nil {
			_ = os.Remove(tmpFile)
		}
		check(writeErr == nil, fmt.Sprintf("GITOPS_DIR is writable (%s)", dir),
			fmt.Sprintf("chmod u+w %s", dir))
	}

	// ── releases/ directory ───────────────────────────────────────────────────
	releasesDir := filepath.Join(dir, "releases")
	_, relErr := os.Stat(releasesDir)
	check(relErr == nil, fmt.Sprintf("releases/ directory exists (%s)", releasesDir),
		fmt.Sprintf("mkdir -p %s", releasesDir))

	// ── manifest.yml ──────────────────────────────────────────────────────────
	mf, mfErr := manifest.ParseFile(manifestPath)
	if mfErr == nil {
		check(true, fmt.Sprintf("manifest.yml is valid (app: %s)", mf.App), "")
	} else {
		check(false, fmt.Sprintf("manifest.yml is invalid: %v", mfErr),
			"run 'runway init' to create a new manifest")
	}

	// ── .env file (if configured) ─────────────────────────────────────────────
	if mf != nil && mf.EnvFile != "" {
		_, envErr := os.Stat(mf.EnvFile)
		check(envErr == nil, fmt.Sprintf(".env file is readable (%s)", mf.EnvFile),
			fmt.Sprintf("touch %s && chmod 600 %s", mf.EnvFile, mf.EnvFile))
	}

	// ── stale lock ────────────────────────────────────────────────────────────
	lockPath := ""
	pid, pidErr := engine.HeldByPID(lockPath)
	if pidErr == nil {
		check(false, fmt.Sprintf("stale lock file found (PID %d)", pid),
			"remove /tmp/runway.lock if no deploy is running")
	} else {
		check(true, "no stale lock file", "")
	}

	// ── git auth (token or SSH key) ───────────────────────────────────────────
	token := gitToken()
	sshKey := false
	for _, candidate := range []string{
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ecdsa"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			sshKey = true
			break
		}
	}
	authOK := token != "" || sshKey
	authMsg := "git auth configured"
	if token != "" {
		authMsg = "git auth configured (GITOPS_GIT_TOKEN is set)"
	} else if sshKey {
		authMsg = "git auth configured (SSH key found)"
	}
	check(authOK, authMsg,
		"set GITOPS_GIT_TOKEN=<token> or add an SSH key (~/.ssh/id_ed25519)")

	// ── summary ───────────────────────────────────────────────────────────────
	fmt.Println()
	if issues == 0 {
		color.Successf(os.Stdout, "all checks passed — runway is ready")
		return nil
	}
	color.Warnf(os.Stdout, "%d issue(s) found — fix the above and re-run 'runway doctor'", issues)
	return fmt.Errorf("%d issue(s) found", issues)
}

func trimNL(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}
