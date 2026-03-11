# Changelog

All notable changes to runway are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
runway uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.5.0] — 2026-03-10

### Added

#### Deploy hooks
- `pre_deploy:` array in `manifest.yml` — commands run before setup/build/start. Any non-zero exit aborts the deploy; the current symlink is not changed.
- `post_deploy:` array in `manifest.yml` — commands run after the service is live. Failures are logged as warnings but **do not revert** the deploy.
- Both hooks run in the release directory with the full deploy environment.
- Audit scanner (`manifest.Audit()`) extended to check `pre_deploy` and `post_deploy` commands for suspicious shell patterns.

#### Zero-downtime deploy (health check)
- `health_check:` block in `manifest.yml` with `url`, `interval` (default 2s), and `retries` (default 10) fields.
- When configured: start commands run first on the new release, then runway polls the URL until HTTP 200. The symlink only swaps once the service is confirmed healthy — the old release stays live throughout.
- If the health check exhausts all retries, the deploy fails and the old release remains active.
- New package: `internal/engine/healthcheck.go` with `waitHealthy()`.

#### Branch-based deploy rules
- `branches:` list in `manifest.yml` — restricts which git branches trigger a webhook deploy.
- Supports exact match (`main`) and trailing wildcard (`release/*`, `feat/*`).
- Empty list = all branches allowed. CLI deploys bypass branch rules.
- Webhook handler now extracts branch from `refs/heads/<branch>` in the push payload `ref` field.
- `engine.Config.Branch` field carries branch info; `manifest.BranchAllowed()` enforces the rules.
- New file: `internal/manifest/branches.go`.

#### Multi-app support
- New `runway.yml` config file in `GITOPS_DIR` lists multiple apps (name, repo, base_dir, branches).
- When `runway.yml` exists, the webhook handler fans out: a parallel deploy is triggered for every app whose `branches:` list matches the pushed branch. Non-matching apps are skipped.
- Each app has its own `base_dir`, `releases/`, `history.json`, and `deploy.log`.
- Single-app mode (env-var driven) unchanged when `runway.yml` is absent.
- New package: `internal/multiapp/` with parser, validation, `BranchAllowed()`.

#### OSS project health
- GitHub Actions CI workflow (`.github/workflows/ci.yml`): test + vet + gofmt check on push/PR, across Go 1.22/1.23, Ubuntu + macOS, with cross-compile check for all 5 target platforms.
- GitHub Actions release workflow (`.github/workflows/release.yml`): builds all platform binaries, generates `SHA256SUMS`, and creates a GitHub Release on `v*.*.*` tag push.
- Issue templates: bug report and feature request (`.github/ISSUE_TEMPLATE/`).
- `CONTRIBUTING.md`: ground rules (no external deps, tests required, no `os.Exit` in packages), project structure map, "how to add a command/field", PR checklist.
- CLI package tests (`internal/cli/cli_test.go`): 26 tests covering dispatch, exit codes, all major commands.

### Changed
- Deploy sequence re-ordered when `health_check.url` is set: start → health check → symlink swap (vs. previous: symlink swap → start).
- `engine.Config` gains a `Branch string` field (zero value = no branch info, bypass rules).
- `manifest.Manifest` gains `PreDeploy`, `PostDeploy`, `HealthCheck`, and `Branches` fields.

## [Unreleased]

### Added
- (nothing yet)

---

## [0.4.0] — 2026-03-10

### Added
- **P4 Security Hardening**
  - Path traversal guard: `env_file` paths are resolved through `filepath.EvalSymlinks` and must fall within `GITOPS_DIR`. Traversal attempts are rejected with a clear error.
  - Command injection audit: `manifest.Audit()` scans `setup`/`build`/`start` commands for suspicious shell patterns (`$(`, `` ` ``, `eval`, `curl`, `wget`, `nc`, `bash -c`, `sh -c`) at deploy time. Warnings are logged to `deploy.log` and stderr. Deploys are not aborted — warnings are advisory.
  - Webhook rate limiting: `/webhook` endpoint uses a token-bucket limiter (default 5 requests/minute). Excess requests receive HTTP 429 with `Retry-After` header. Configurable via `--webhook-rate-limit` flag (`0` = unlimited).

### Changed
- `runway listen` now accepts `--webhook-rate-limit N` flag (default: 5)

---

## [0.3.0] — 2026-03-09

### Added
- **P3 Observability**
  - Email notifications via `net/smtp` (stdlib, no external deps). Configure `notify.email` block in `manifest.yml`. SMTP password read from `RUNWAY_SMTP_PASSWORD` env var.
  - Structured JSON event logging: `runway listen --log-format json` emits newline-delimited JSON to stderr for log aggregators. Text format remains the default.
  - Typed exit codes: `deploy` and `rollback` return distinct exit codes (0–7) for scripting and CI integration. `main.go` uses `cli.ExitCode(err)` instead of hardcoded `os.Exit(1)`.

---

## [0.2.0] — 2026-03-08

### Added
- **P2 Developer Experience**
  - ANSI terminal colours: green ✓ success, red ✗ error, yellow ⚠ warning, cyan → info. Disabled automatically for non-TTY output. Also respects `NO_COLOR` env var and `--no-color` global flag.
  - Streaming deploy output: CLI-triggered deploys stream `[runway]`-prefixed output to the terminal in real time while also writing to `deploy.log`.
  - `runway init`: interactive setup wizard that detects your runtime (Node.js, Python, Go, Ruby) and generates `manifest.yml`, `releases/`, and `.env`.
  - `runway doctor`: 9-point setup health check — git binary, env vars, directory structure, manifest validity, `.env` access, stale lock, git auth.
  - `runway deploy --dry-run`: validates config and prints commands that would run, without executing anything.
  - `runway history`: shows full deployment history with `--limit N` and `--status <filter>` flags.
  - `timeout:` field in `manifest.yml` — setup+build phases are killed after this many seconds (default 600). Start commands always get a separate 2-minute window.

### Changed
- `runway deploy` and `runway rollback` now use coloured output for success/failure messages.
- `runway status` and `runway releases` use coloured active-release indicators.
- `runway listen` prints graceful-shutdown message on `SIGINT`/`SIGTERM`.

---

## [0.1.0] — 2026-03-07

### Added
- **P1 Safety & Reliability**
  - Deploy timeout with `context.WithTimeout` + `exec.CommandContext` — long-running commands are killed cleanly.
  - Graceful shutdown of webhook listener: `SIGINT`/`SIGTERM` stops accepting new connections, waits up to 10s for HTTP drain, then waits for any in-flight deploy to complete.
  - Auto-rollback on start failure: if `start` commands fail after the symlink has been switched, runway automatically reverts to the previous release and records the event.
  - Stale lock detection: if `AcquireLock` finds a lock held by a dead process (via `kill(pid, 0)`), it reports a diagnostic rather than hanging.
  - `history.json` backup: a `.bak` copy is written before every history update. `Load()` falls back to `.bak` transparently if the live file is corrupt.

### Added (initial release)
- Single binary deployment manager — `runway deploy`, `runway rollback`, `runway status`, `runway releases`, `runway log`, `runway listen`, `runway version`.
- `manifest.yml` — app name, env_file, setup/build/start command lists.
- HMAC-SHA256 webhook signature verification (`X-Hub-Signature-256`).
- File lock (`syscall.Flock` on Linux/macOS, `O_EXCL` PID file on Windows) preventing concurrent deploys.
- Atomic symlink swap via `os.Symlink` + `os.Rename`.
- Deployment history in `history.json` (last 15 entries).
- Per-deploy log at `releases/<commit>/deploy.log`.
- Automatic cleanup of old releases (keeps last 15).
- SSH and HTTPS token authentication for private repositories.
- systemd service file in `systemd/runway.service`.
- Cross-platform release binaries: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64.

---

[Unreleased]: https://github.com/Reeteshrajesh/runway/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/Reeteshrajesh/runway/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Reeteshrajesh/runway/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Reeteshrajesh/runway/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Reeteshrajesh/runway/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Reeteshrajesh/runway/releases/tag/v0.1.0
