package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/Reeteshrajesh/runway/internal/envloader"
	"github.com/Reeteshrajesh/runway/internal/logger"
	"github.com/Reeteshrajesh/runway/internal/manifest"
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
}

// DeployResult captures the outcome of a deployment attempt.
type DeployResult struct {
	Commit    string
	Success   bool
	StartedAt time.Time
	EndedAt   time.Time
	LogPath   string
	Err       error
}

// Deploy runs a full deployment for the given config.
//
// Deployment sequence:
//  1. Acquire exclusive lock (fail fast if another deploy is running)
//  2. Clone repository at target commit
//  3. Parse manifest.yml from cloned repo
//  4. Load .env file
//  5. Run setup commands
//  6. Run build commands
//  7. Create release directory, move clone into it
//  8. Update current symlink (atomic)
//  9. Run start commands
//  10. Update history.json
//  11. Release lock
//
// On any failure after step 2, the partial release directory is removed
// and the current symlink is left unchanged.
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
			// Non-fatal: log to stderr but do not override the deploy result.
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

	// From this point on, clean up the release directory on failure.
	deployFailed := true
	defer func() {
		if deployFailed {
			_ = mgr.RemoveReleaseDir(cfg.Commit)
		}
	}()

	// ── Step 3: Open deploy logger ────────────────────────────────────────────
	log, err := logger.New(releaseDir, os.Stdout)
	if err != nil {
		result.Err = fmt.Errorf("open deploy log: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}
	defer log.Close()
	result.LogPath = log.Path()

	log.Logf("=== GitOps Lite Deploy ===")
	log.Logf("commit:    %s", cfg.Commit)
	log.Logf("triggered: %s", cfg.Triggered)
	log.Logf("base dir:  %s", cfg.BaseDir)

	// ── Step 4: Clone repository ──────────────────────────────────────────────
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
		_ = hist.Append(release.Deployment{
			Commit:    cfg.Commit,
			Time:      result.EndedAt,
			Status:    release.StatusFailed,
			Triggered: cfg.Triggered,
		})
		return result
	}

	// ── Step 5: Parse manifest ────────────────────────────────────────────────
	log.Logf("--- parsing manifest ---")
	manifestPath := releaseDir + "/manifest.yml"
	mf, err := manifest.ParseFile(manifestPath)
	if err != nil {
		result.Err = fmt.Errorf("manifest: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}
	log.Logf("app: %s", mf.App)

	// ── Step 6: Load environment ──────────────────────────────────────────────
	envPairs, err := loadEnv(cfg.BaseDir, mf.EnvFile)
	if err != nil {
		result.Err = fmt.Errorf("env: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	runOpts := RunOptions{
		Dir:    releaseDir,
		Env:    envPairs,
		Stdout: log,
		Stderr: log,
	}

	// ── Step 7: Run setup commands ────────────────────────────────────────────
	if len(mf.Setup) > 0 {
		log.Logf("--- running setup ---")
		if err := RunCommands(mf.Setup, runOpts); err != nil {
			result.Err = fmt.Errorf("setup: %w", err)
			result.EndedAt = time.Now().UTC()
			_ = hist.Append(release.Deployment{
				Commit:    cfg.Commit,
				Time:      result.EndedAt,
				Status:    release.StatusFailed,
				Triggered: cfg.Triggered,
			})
			return result
		}
	}

	// ── Step 8: Run build commands ────────────────────────────────────────────
	if len(mf.Build) > 0 {
		log.Logf("--- running build ---")
		if err := RunCommands(mf.Build, runOpts); err != nil {
			result.Err = fmt.Errorf("build: %w", err)
			result.EndedAt = time.Now().UTC()
			_ = hist.Append(release.Deployment{
				Commit:    cfg.Commit,
				Time:      result.EndedAt,
				Status:    release.StatusFailed,
				Triggered: cfg.Triggered,
			})
			return result
		}
	}

	// ── Step 9: Update symlink (atomic) ───────────────────────────────────────
	log.Logf("--- updating current symlink ---")
	if err := mgr.UpdateCurrent(cfg.Commit); err != nil {
		result.Err = fmt.Errorf("symlink: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// ── Step 10: Run start commands ───────────────────────────────────────────
	log.Logf("--- starting service ---")
	if err := RunCommands(mf.Start, runOpts); err != nil {
		result.Err = fmt.Errorf("start: %w", err)
		result.EndedAt = time.Now().UTC()
		_ = hist.Append(release.Deployment{
			Commit:    cfg.Commit,
			Time:      result.EndedAt,
			Status:    release.StatusFailed,
			Triggered: cfg.Triggered,
		})
		return result
	}

	// ── Step 11: Cleanup old releases ────────────────────────────────────────
	log.Logf("--- cleaning up old releases ---")
	if err := mgr.Cleanup(cfg.Commit); err != nil {
		// Non-fatal: log but don't fail the deploy.
		log.Logf("warning: cleanup failed: %v", err)
	}

	// ── Step 12: Record success ───────────────────────────────────────────────
	result.Success = true
	result.EndedAt = time.Now().UTC()
	deployFailed = false

	_ = hist.Append(release.Deployment{
		Commit:    cfg.Commit,
		Time:      result.EndedAt,
		Status:    release.StatusRunning,
		Triggered: cfg.Triggered,
	})

	log.Logf("=== deploy complete: %s ===", cfg.Commit)
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

	// ── Step 3: Update symlink ────────────────────────────────────────────────
	if err := mgr.UpdateCurrent(cfg.Commit); err != nil {
		result.Err = fmt.Errorf("symlink: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// ── Step 4: Parse manifest and restart service ────────────────────────────
	releaseDir := mgr.ReleaseDir(cfg.Commit)
	mf, err := manifest.ParseFile(releaseDir + "/manifest.yml")
	if err != nil {
		result.Err = fmt.Errorf("manifest: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	envPairs, err := loadEnv(cfg.BaseDir, mf.EnvFile)
	if err != nil {
		result.Err = fmt.Errorf("env: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	runOpts := RunOptions{
		Dir:    releaseDir,
		Env:    envPairs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := RunCommands(mf.Start, runOpts); err != nil {
		result.Err = fmt.Errorf("start: %w", err)
		result.EndedAt = time.Now().UTC()
		return result
	}

	// ── Step 5: Record rollback in history ───────────────────────────────────
	result.Success = true
	result.EndedAt = time.Now().UTC()

	_ = hist.Append(release.Deployment{
		Commit:    cfg.Commit,
		Time:      result.EndedAt,
		Status:    release.StatusRolledBack,
		Triggered: cfg.Triggered,
	})

	return result
}

// loadEnv loads the .env file referenced in the manifest, resolved relative
// to the base directory. Returns merged os.Environ() + .env pairs.
func loadEnv(baseDir, envFile string) ([]string, error) {
	if envFile == "" {
		return envloader.Merge(os.Environ(), nil), nil
	}

	// Resolve envFile relative to base dir (not the release dir).
	path := envFile
	if !isAbsPath(envFile) {
		path = baseDir + "/" + envFile
	}

	pairs, err := envloader.Load(path)
	if err != nil {
		return nil, err
	}
	return envloader.Merge(os.Environ(), pairs), nil
}

func isAbsPath(p string) bool {
	return len(p) > 0 && p[0] == '/'
}
