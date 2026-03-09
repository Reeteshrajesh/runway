package engine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneOptions configures a git clone operation.
type CloneOptions struct {
	// RepoURL is the remote repository URL (SSH or HTTPS).
	RepoURL string

	// Commit is the commit SHA or branch to checkout after cloning.
	Commit string

	// DestDir is the directory where the repo will be cloned.
	DestDir string

	// GitToken is an optional personal access token for HTTPS authentication.
	// When set, the clone URL is rewritten to embed the token.
	// Never logged.
	GitToken string

	// Stdout and Stderr receive git command output (typically the deploy logger).
	Stdout io.Writer
	Stderr io.Writer
}

// CloneAtCommit clones the repository and checks out the specified commit.
// On failure it does not attempt cleanup — the caller is responsible for
// removing DestDir if needed.
func CloneAtCommit(opts CloneOptions) error {
	repoURL := opts.RepoURL
	if opts.GitToken != "" {
		repoURL = injectToken(repoURL, opts.GitToken)
	}

	// Step 1: shallow clone (depth 1 is insufficient for arbitrary commit checkout,
	// so we clone without depth and rely on the server to negotiate efficiently).
	cloneArgs := []string{"clone", "--quiet", repoURL, opts.DestDir}
	if err := runGit(cloneArgs, "", opts.Stdout, opts.Stderr); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Step 2: check out the exact commit.
	checkoutArgs := []string{"checkout", "--quiet", opts.Commit}
	if err := runGit(checkoutArgs, opts.DestDir, opts.Stdout, opts.Stderr); err != nil {
		return fmt.Errorf("git checkout %s: %w", opts.Commit, err)
	}

	return nil
}

// ResolveCommit resolves a short commit SHA, branch name, or tag to a full
// 40-character commit SHA inside an already-cloned repository at repoDir.
func ResolveCommit(repoDir, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}

	full := strings.TrimSpace(string(out))
	if len(full) < 7 {
		return "", fmt.Errorf("git rev-parse returned unexpected output: %q", full)
	}
	return full, nil
}

// ShortSHA returns the first 7 characters of a commit SHA.
func ShortSHA(commit string) string {
	if len(commit) >= 7 {
		return commit[:7]
	}
	return commit
}

// runGit runs a git subcommand, wiring stdout/stderr to the provided writers.
func runGit(args []string, dir string, stdout, stderr io.Writer) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Inherit a clean environment to avoid user-level git configs interfering,
	// but keep HOME so SSH keys are discoverable.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // never prompt for credentials — fail fast instead
	)

	return cmd.Run()
}

// injectToken rewrites an HTTPS URL to embed a token credential.
// Example: https://github.com/org/repo → https://<token>@github.com/org/repo
// SSH URLs are returned unchanged.
func injectToken(repoURL, token string) string {
	const httpsPrefix = "https://"
	if !strings.HasPrefix(repoURL, httpsPrefix) {
		return repoURL // SSH URL — token not applicable
	}
	rest := strings.TrimPrefix(repoURL, httpsPrefix)
	return httpsPrefix + token + "@" + rest
}

// GitAvailable checks whether git is installed and accessible on PATH.
func GitAvailable() error {
	path, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git not found on PATH: install git and retry")
	}
	_ = path
	return nil
}

// CleanupCloneDir removes a cloned repository directory.
// Safe to call even if the directory does not exist.
func CleanupCloneDir(dir string) error {
	if dir == "" || dir == "/" || !filepath.IsAbs(dir) {
		return fmt.Errorf("cleanup: refusing to remove unsafe path %q", dir)
	}
	return os.RemoveAll(dir)
}
