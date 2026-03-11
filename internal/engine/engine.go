package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	"bufio"
	"github.com/Reeteshrajesh/runway/internal/envloader"
	"github.com/Reeteshrajesh/runway/internal/logger"
	"path/filepath"
	"strings"

	"github.com/Reeteshrajesh/runway/internal/manifest"
	"github.com/Reeteshrajesh/runway/internal/notify"
	"github.com/Reeteshrajesh/runway/internal/release"
)

// Config holds all runtime configuration for a deployment.
type Config struct {
	// BaseDir is the runway working directory (e.g. /opt/runway).
	BaseDir string

	// RepoURL is the remote git repository URL.
	RepoURL string

	// Commit is the target commit SHA or ref to deploy.
	Commit string

	// GitToken is an optional HTTPS auth token (never logged).
	GitToken string

	// LockPath overrides the default lock file path (useful for testing).
	LockPath string

	// Triggered describes what initiated the deploy ("cli" or "webhook").
	Triggered string

	// Branch is the git branch that triggered the deploy (optional).
	// Set by the webhook handler from the push payload ref.
	// Used for branch-based deploy rules (branches: in manifest.yml).
	Branch string
}

// DeployResult captures the outcome of a deployment attempt.
type DeployResult struct {
	Commit         string
	Success        bool
	StartedAt      time.Time
	EndedAt        time.Time
	LogPath        string
	AutoRolledBack bool   // true if start failed and we rolled back automatically
	RolledBackTo   string // commit we rolled back to (if AutoRolledBack)
	Err            error
}

// Deploy runs a full deployment for the given config.
//
// Deployment sequence:
//  1. Acquire exclusive lock (fail fast if another deploy is running)
//  2. Create release directory
//  3. Open deploy logger
//  4. Clone repository at target commit
//  5. Parse manifest.yml (also reads timeout)
//  6. Load .env file
//  7. Run setup commands (within timeout context)
//  8. Run build commands (within timeout context)
//  9. Update current symlink (atomic)
//  10. Run start commands — on failure: auto-rollback to previous release
//  11. Cleanup old releases
//  12. Record result in history.json
//
// On any failure before step 9 the partial release directory is removed and
// the current symlink is left unchanged.
func Deploy(cfg Config) DeployResult {
	result := DeployResult{
		Commit:    cfg.Commit,
		StartedAt: time.Now().UTC(),
	}

	mgr := release.NewManager(cfg.BaseDir)
	hist := release.NewHistory(cfg.BaseDir)

	// ── Step 1: Acquire lock ──────────────────────────────────────────────────
	lock, err := AcquireLock(cfg.LockPath)
	if err != nil {
		result.Err = err
		result.EndedAt = time.Now().UTC()
		return result
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to release lock: %v\n", releaseErr)
		}
	}()

	// ── Step 2: Create release directory ─────────────────────────────────────
	releaseDir := mgr.ReleaseDir(cfg.Commit)
	if err := mgr.CreateReleaseDir(cfg.Commit); err != nil {
		result.Err = fmt.Errorf("create release dir: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// From this point on, clean up the release directory on failure
	// (unless the deploy succeeded or we already switched symlinks).
	symlinkSwitched := false
	deployFailed := true
	defer func() {
		if deployFailed && !symlinkSwitched {
			_ = mgr.RemoveReleaseDir(cfg.Commit)
		}
	}()

	// ── Step 3: Open deploy logger ────────────────────────────────────────────
	// CLI-triggered deploys stream prefixed output to the terminal in real time.
	// Webhook-triggered deploys write only to deploy.log (no watching operator).
	var logErr error
	var log *logger.DeployLogger
	if cfg.Triggered == "cli" {
		log, logErr = logger.NewStreaming(releaseDir, "[runway] ", os.Stdout)
	} else {
		log, logErr = logger.New(releaseDir, nil)
	}
	if logErr != nil {
		result.Err = fmt.Errorf("open deploy log: %w", logErr)
		result.EndedAt = time.Now().UTC()
		return result
	}
	defer log.Close()
	result.LogPath = log.Path()

	log.Logf("=== runway deploy ===")
	log.Logf("commit:    %s", cfg.Commit)
	log.Logf("triggered: %s", cfg.Triggered)
	log.Logf("base dir:  %s", cfg.BaseDir)

	// ── Step 4: Clone repository ──────────────────────────────────────────────
	log.Step("\n→ cloning repository")
	log.Logf("--- cloning repository ---")
	if err := CloneAtCommit(CloneOptions{
		RepoURL:  cfg.RepoURL,
		Commit:   cfg.Commit,
		DestDir:  releaseDir,
		GitToken: cfg.GitToken,
		Stdout:   log,
		Stderr:   log,
	}); err != nil {
		result.Err = fmt.Errorf("clone: %w", err)
		result.EndedAt = time.Now().UTC()
		recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
		return result
	}

	// ── Step 5: Parse manifest ────────────────────────────────────────────────
	log.Step("→ parsing manifest")
	log.Logf("--- parsing manifest ---")
	mf, err := manifest.ParseFile(releaseDir + "/manifest.yml")
	if err != nil {
		result.Err = fmt.Errorf("manifest: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}
	log.Logf("app:     %s", mf.App)
	log.Logf("timeout: %ds", mf.TimeoutSeconds)

	// ── Branch rule check ───────────────────────────────────────────────────
	// Enforce branches: filter only when triggered by webhook (has branch info).
	if cfg.Branch != "" && !mf.BranchAllowed(cfg.Branch) {
		log.Logf("branch %q is not in the allowed list — skipping deploy", cfg.Branch)
		result.Err = fmt.Errorf("branch %q is not in the allowed branches list: %v", cfg.Branch, mf.Branches)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// ── P4.2: Audit manifest commands for suspicious shell patterns ───────────
	for _, w := range mf.Audit() {
		log.Logf("WARNING: %s", w)
		fmt.Fprintf(os.Stderr, "runway: WARNING: %s\n", w)
	}

	// ── Step 6: Load environment ──────────────────────────────────────────────
	envPairs, err := loadEnv(cfg.BaseDir, mf.EnvFile)
	if err != nil {
		result.Err = fmt.Errorf("env: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// ── Build/setup context with timeout ─────────────────────────────────────
	// The timeout covers setup + build only. Start gets its own short timeout.
	buildCtx, cancelBuild := context.WithTimeout(
		context.Background(),
		time.Duration(mf.TimeoutSeconds)*time.Second,
	)
	defer cancelBuild()

	runOpts := RunOptions{
		Dir:    releaseDir,
		Env:    envPairs,
		Stdout: log,
		Stderr: log,
	}

	// ── Step 7: Run pre_deploy hooks ─────────────────────────────────────────
	// pre_deploy runs before setup/build/start. Failure aborts the deploy.
	if len(mf.PreDeploy) > 0 {
		log.Step("→ running pre_deploy hooks (%d commands)", len(mf.PreDeploy))
		log.Logf("--- running pre_deploy ---")
		if err := RunCommands(buildCtx, mf.PreDeploy, runOpts); err != nil {
			result.Err = fmt.Errorf("pre_deploy: %w", err)
			result.EndedAt = time.Now().UTC()
			recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
			return result
		}
	}

	// ── Step 9: Run setup commands ────────────────────────────────────────────
	if len(mf.Setup) > 0 {
		log.Step("→ running setup (%d commands)", len(mf.Setup))
		log.Logf("--- running setup ---")
		if err := RunCommands(buildCtx, mf.Setup, runOpts); err != nil {
			result.Err = fmt.Errorf("setup: %w", err)
			result.EndedAt = time.Now().UTC()
			recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
			return result
		}
	}

	// ── Step 10: Run build commands ───────────────────────────────────────────
	if len(mf.Build) > 0 {
		log.Step("→ running build (%d commands)", len(mf.Build))
		log.Logf("--- running build ---")
		if err := RunCommands(buildCtx, mf.Build, runOpts); err != nil {
			result.Err = fmt.Errorf("build: %w", err)
			result.EndedAt = time.Now().UTC()
			recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
			return result
		}
	}
	cancelBuild() // release the context early; build is done

	// ── Snapshot previous active commit before switching symlink ─────────────
	// Used for auto-rollback if start fails (P1.3).
	previousCommit, _ := mgr.ActiveCommit()

	// ── Step 11: Run start commands ───────────────────────────────────────────
	// When health_check is configured, start runs BEFORE the symlink swap so
	// the new release can be validated while the old one is still live
	// (zero-downtime). Without health_check the symlink swaps first (original
	// behaviour) for backwards compatibility.
	log.Step("→ starting service")
	log.Logf("--- starting service ---")
	startCtx, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	if err := RunCommands(startCtx, mf.Start, runOpts); err != nil {
		log.Logf("start failed: %v", err)

		// If symlink has not been switched yet (health_check flow), clean up
		// the release dir; otherwise attempt auto-rollback.
		if !symlinkSwitched {
			result.Err = fmt.Errorf("start failed (before symlink swap): %w", err)
			result.EndedAt = time.Now().UTC()
			recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
			return result
		}

		// ── Auto-rollback (P1.3) ──────────────────────────────────────────────
		if previousCommit != "" && previousCommit != cfg.Commit {
			log.Logf("--- auto-rolling back to %s ---", previousCommit)
			rbErr := performRollback(mgr, hist, cfg, previousCommit, log)
			if rbErr != nil {
				log.Logf("auto-rollback also failed: %v", rbErr)
				result.Err = fmt.Errorf("start failed AND auto-rollback failed: start=%w rollback=%v", err, rbErr)
			} else {
				result.AutoRolledBack = true
				result.RolledBackTo = previousCommit
				result.Err = fmt.Errorf("start failed — automatically rolled back to %s: %w", previousCommit, err)
			}
		} else {
			result.Err = fmt.Errorf("start: %w", err)
		}

		result.EndedAt = time.Now().UTC()
		recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
		return result
	}

	// ── Step 12: Health check (zero-downtime) ─────────────────────────────────
	// If health_check.url is configured, poll it now. The old symlink is still
	// live while we wait. Only swap once the new release is confirmed healthy.
	if mf.HealthCheck.URL != "" {
		log.Step("→ waiting for health check (%s)", mf.HealthCheck.URL)
		log.Logf("--- health check ---")
		if err := waitHealthy(mf.HealthCheck, log.Logf); err != nil {
			log.Logf("health check failed: %v — aborting deploy (old release still active)", err)
			result.Err = fmt.Errorf("health check: %w", err)
			result.EndedAt = time.Now().UTC()
			recordHistory(hist, cfg, release.StatusFailed, result.EndedAt)
			return result
		}
	}

	// ── Step 13: Update symlink (atomic) ──────────────────────────────────────
	log.Step("→ updating current symlink")
	log.Logf("--- updating current symlink ---")
	if err := mgr.UpdateCurrent(cfg.Commit); err != nil {
		result.Err = fmt.Errorf("symlink: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}
	symlinkSwitched = true

	// ── Step 13: Run post_deploy hooks ───────────────────────────────────────
	// post_deploy runs after service is live. Failure is logged but does NOT
	// revert the deploy — the service is already running successfully.
	if len(mf.PostDeploy) > 0 {
		log.Step("→ running post_deploy hooks (%d commands)", len(mf.PostDeploy))
		log.Logf("--- running post_deploy ---")
		postCtx, cancelPost := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancelPost()
		if err := RunCommands(postCtx, mf.PostDeploy, runOpts); err != nil {
			// Non-fatal: service is running; just log the warning.
			log.Logf("WARNING: post_deploy hook failed (service is still running): %v", err)
			fmt.Fprintf(os.Stderr, "runway: WARNING: post_deploy hook failed: %v\n", err)
		}
	}

	// ── Step 14: Cleanup old releases ────────────────────────────────────────
	log.Step("→ cleaning up old releases")
	log.Logf("--- cleaning up old releases ---")
	if err := mgr.Cleanup(cfg.Commit); err != nil {
		log.Logf("warning: cleanup failed: %v", err)
	}

	// ── Step 15: Record success ───────────────────────────────────────────────
	result.Success = true
	result.EndedAt = time.Now().UTC()
	deployFailed = false

	recordHistory(hist, cfg, release.StatusRunning, result.EndedAt)
	log.Logf("=== deploy complete: %s (%.1fs) ===",
		cfg.Commit, result.EndedAt.Sub(result.StartedAt).Seconds())
	sendNotification(mf, cfg, result)
	return result
}

// Rollback switches the active release to a previously deployed commit.
func Rollback(cfg Config) DeployResult {
	result := DeployResult{
		Commit:    cfg.Commit,
		StartedAt: time.Now().UTC(),
	}

	mgr := release.NewManager(cfg.BaseDir)
	hist := release.NewHistory(cfg.BaseDir)

	// ── Step 1: Verify release exists ────────────────────────────────────────
	releases, err := mgr.ListReleases()
	if err != nil {
		result.Err = fmt.Errorf("list releases: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}
	found := false
	for _, r := range releases {
		if r == cfg.Commit {
			found = true
			break
		}
	}
	if !found {
		result.Err = fmt.Errorf("release %q not found — run 'runway releases' to see available releases", cfg.Commit)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// ── Step 2: Acquire lock ──────────────────────────────────────────────────
	lock, err := AcquireLock(cfg.LockPath)
	if err != nil {
		result.Err = err
		result.EndedAt = time.Now().UTC()
		return result
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to release lock: %v\n", releaseErr)
		}
	}()

	// ── Steps 3–5: Switch symlink + restart ───────────────────────────────────
	if err := performRollback(mgr, hist, cfg, cfg.Commit, nil); err != nil {
		result.Err = err
		result.EndedAt = time.Now().UTC()
		return result
	}

	result.Success = true
	result.EndedAt = time.Now().UTC()
	return result
}

// performRollback switches the symlink to targetCommit, restarts the service,
// and records the event in history. Used by both Rollback and auto-rollback.
// log may be nil (auto-rollback during deploy writes to the deploy log instead).
func performRollback(
	mgr *release.Manager,
	hist *release.HistoryManager,
	cfg Config,
	targetCommit string,
	log interface{ Logf(string, ...any) },
) error {
	logf := func(format string, args ...any) {
		if log != nil {
			log.Logf(format, args...)
		} else {
			fmt.Fprintf(os.Stderr, "[runway] "+format+"\n", args...)
		}
	}

	if err := mgr.UpdateCurrent(targetCommit); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	releaseDir := mgr.ReleaseDir(targetCommit)
	mf, err := manifest.ParseFile(releaseDir + "/manifest.yml")
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	envPairs, err := loadEnv(cfg.BaseDir, mf.EnvFile)
	if err != nil {
		return fmt.Errorf("env: %w", err)
	}

	logf("restarting service from %s", targetCommit)
	startCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := RunCommands(startCtx, mf.Start, RunOptions{
		Dir:    releaseDir,
		Env:    envPairs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	_ = hist.Append(release.Deployment{
		Commit:    targetCommit,
		Time:      time.Now().UTC(),
		Status:    release.StatusRolledBack,
		Triggered: cfg.Triggered,
	})

	return nil
}

// recordHistory writes a deployment event to history, ignoring errors
// (history failure must never abort a deploy).
func recordHistory(hist *release.HistoryManager, cfg Config, status release.DeployStatus, t time.Time) {
	_ = hist.Append(release.Deployment{
		Commit:    cfg.Commit,
		Time:      t,
		Status:    status,
		Triggered: cfg.Triggered,
	})
}

// loadEnv loads the .env file referenced in the manifest, resolved relative
// to the base directory. Returns merged os.Environ() + .env pairs.
// Guards against path traversal: env_file must resolve inside baseDir.
func loadEnv(baseDir, envFile string) ([]string, error) {
	if envFile == "" {
		return envloader.Merge(os.Environ(), nil), nil
	}

	path := envFile
	if !isAbsPath(envFile) {
		path = baseDir + "/" + envFile
	}

	if err := guardPath(baseDir, path); err != nil {
		return nil, err
	}

	pairs, err := envloader.Load(path)
	if err != nil {
		return nil, err
	}
	return envloader.Merge(os.Environ(), pairs), nil
}

// guardPath verifies that target resolves to a path inside baseDir after
// evaluating symlinks. Returns an error on any traversal attempt.
func guardPath(baseDir, target string) error {
	// Resolve symlinks so "../../../etc/passwd" tricks are caught.
	// If the file doesn't exist yet we resolve the directory instead.
	resolvedBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		// baseDir doesn't exist — use the raw path for the prefix check.
		resolvedBase = filepath.Clean(baseDir)
	}

	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		// File may not exist yet; clean and check structurally.
		resolvedTarget = filepath.Clean(target)
	}

	if !strings.HasPrefix(resolvedTarget, resolvedBase+string(filepath.Separator)) &&
		resolvedTarget != resolvedBase {
		return fmt.Errorf("env_file path %q escapes GITOPS_DIR %q — path traversal rejected", target, baseDir)
	}
	return nil
}

func isAbsPath(p string) bool {
	return len(p) > 0 && p[0] == '/'
}

// sendNotification fires a best-effort email notification after a deploy.
// Errors are logged to stderr but never abort the deploy result.
func sendNotification(mf *manifest.Manifest, cfg Config, result DeployResult) {
	if mf == nil || mf.Notify.To == "" {
		return
	}
	ev := notify.DeployEvent{
		App:          mf.App,
		Commit:       result.Commit,
		Triggered:    cfg.Triggered,
		Duration:     result.EndedAt.Sub(result.StartedAt),
		Err:          result.Err,
		RolledBack:   result.AutoRolledBack,
		RolledBackTo: result.RolledBackTo,
	}
	if result.Err != nil && result.LogPath != "" {
		ev.LastLogLines = tailFile(result.LogPath, 20)
	}
	if err := notify.SendDeployEmail(mf.Notify, ev); err != nil {
		fmt.Fprintf(os.Stderr, "notify: failed to send email: %v\n", err)
	}
}

// tailFile reads the last n lines of a file. Returns nil on any error.
func tailFile(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	// Strip ANSI / binary noise that might appear in log lines.
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
