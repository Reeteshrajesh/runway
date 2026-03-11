# runway

> Lightweight git-based deployment manager for single servers.
> No Docker. No Kubernetes. Just `git push` → deploy → rollback in ~1s.

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Reeteshrajesh/runway?style=flat)](https://github.com/Reeteshrajesh/runway/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Reeteshrajesh/runway)](https://goreportcard.com/report/github.com/Reeteshrajesh/runway)

---

## What is runway?

**runway** is a single binary that turns any Linux server into a deployment target.

You push code to Git → runway deploys it automatically. If something breaks, rollback takes ~1 second — no rebuild, no downtime dance.

It is designed for teams who want Git-triggered deployments **without** the weight of Docker, Kubernetes, or cloud-specific tooling.

```
git push origin main
  │
  ▼
runway (webhook listener on your server)
  │
  ├── pulls the commit
  ├── runs your manifest (setup → build → start)
  ├── stores release in releases/<commit>/
  └── updates symlink: current → releases/<commit>
```

**Rollback** just flips the symlink back. No rebuilding. ~1 second.

---

## Why runway?

| | runway | Capistrano | Dokku | Kamal | GitHub Actions |
|---|---|---|---|---|---|
| Docker required | ❌ | ❌ | ✅ | ✅ | optional |
| Runtime on server | ❌ | Ruby | Docker | Docker | runner agent |
| Single binary | ✅ | ❌ | ❌ | ❌ | ❌ |
| Instant rollback | ✅ | ✅ | ✅ | ✅ | ❌ |
| Webhook triggered | ✅ | ❌ | ✅ | ❌ | ✅ |
| Zero config infra | ✅ | ❌ | ❌ | ❌ | ❌ |

---

## Features

- **Commit-based deployments** — every deploy is tied to a specific git commit
- **Instant rollback** — switch to any previous release in ~1 second, no rebuild
- **Auto-rollback** — if your service fails to start, runway automatically reverts to the last good release
- **Manifest-driven** — define setup, build, and start commands in `manifest.yml`
- **Webhook listener** — receives GitHub/GitLab push events with HMAC-SHA256 verification
- **Rate limiting** — webhook endpoint accepts at most N requests per minute (configurable, default 5)
- **Deploy timeout** — build/setup steps are killed after a configurable timeout (default 10 minutes)
- **Concurrent deploy protection** — file lock prevents overlapping deployments, with stale lock detection
- **Build failure safety** — broken builds are deleted before going live; symlink never updated
- **Per-deploy logs** — full stdout/stderr saved to `releases/<commit>/deploy.log`
- **Streaming output** — CLI deploys stream `[runway]`-prefixed output to your terminal in real time
- **Environment variable support** — inject secrets via `.env` file, never stored in git
- **Path traversal guard** — `env_file` paths that escape `GITOPS_DIR` are rejected
- **Command injection audit** — suspicious shell patterns in manifest commands trigger warnings at deploy time
- **Deployment history** — tracks last 15 deployments in `history.json` with automatic `.bak` recovery
- **Email notifications** — get notified on deploy success, failure, or auto-rollback via SMTP
- **Structured JSON logging** — webhook events can be logged as newline-delimited JSON for log aggregators
- **Colored output** — green/red/yellow status with `--no-color` and `NO_COLOR` support
- **Typed exit codes** — distinct exit codes for lock conflicts, build failures, git errors, and more
- **Interactive setup** — `runway init` detects your runtime and generates `manifest.yml`
- **Doctor command** — `runway doctor` checks your entire setup and tells you exactly what's wrong
- **Dry-run mode** — `runway deploy --dry-run` validates config and prints commands without executing
- **Deploy hooks** — `pre_deploy` commands abort a deploy early; `post_deploy` commands run after service is live
- **Zero-downtime deploy** — `health_check:` polls your service URL before swapping the symlink; old release stays live until healthy
- **Branch rules** — `branches:` list restricts which git branches trigger a deploy (exact match + `*` wildcard)
- **Multi-app support** — a single runway instance manages multiple services via `runway.yml` in `GITOPS_DIR`
- **Single binary** — copy one file to your server, done. Zero external dependencies.

---

## Install

### Option 1 — curl (Linux, recommended for servers)

```bash
curl -sSL https://github.com/Reeteshrajesh/runway/releases/latest/download/runway-linux-amd64 \
  -o /usr/local/bin/runway && chmod +x /usr/local/bin/runway
```

For ARM64 servers:

```bash
curl -sSL https://github.com/Reeteshrajesh/runway/releases/latest/download/runway-linux-arm64 \
  -o /usr/local/bin/runway && chmod +x /usr/local/bin/runway
```

### Option 2 — go install

```bash
go install github.com/Reeteshrajesh/runway/cmd/runway@latest
```

### Option 3 — Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/Reeteshrajesh/runway.git
cd runway
make install
```

### Verify installation

```bash
runway version
```

---

## Quick Start

### 1. Initialize on the server

The fastest way to get going is `runway init`. Run it on your server inside your project directory:

```bash
runway init
```

It detects your runtime (Node.js, Python, Go, Ruby), asks a few questions, and writes `manifest.yml`, `releases/`, and an empty `.env` in one step. Then follow the printed "Next steps".

Or set up manually:

### 2. Create the server directory

```bash
sudo mkdir -p /opt/runway/releases
sudo chown -R deploy:deploy /opt/runway
```

> Replace `deploy` with the user that will run runway on your server.

### 3. Create `manifest.yml` in your project root

```yaml
app: my-service

env_file: .env

setup:
  - npm install

build:
  - npm run build

start:
  - pm2 restart my-service || pm2 start dist/index.js --name my-service
```

Commit this file to your repository. runway reads it from the cloned repo during each deployment.

### 4. Create `.env` on the server (not in git)

```bash
# /opt/runway/.env
DATABASE_URL=postgres://localhost/mydb
API_KEY=your-secret-key
NODE_ENV=production
```

### 5. Set environment variables for runway

```bash
export GITOPS_REPO=git@github.com:yourorg/your-repo.git
export GITOPS_DIR=/opt/runway
export GITOPS_WEBHOOK_SECRET=your-webhook-secret
```

> For permanent setup, add these to `/etc/runway/env` (see systemd section below).

### 6. Verify your setup

```bash
runway doctor
```

### 7. Test a manual deploy

```bash
runway deploy <commit-sha>
```

### 8. Start the webhook listener

```bash
runway listen --port 9000 --secret your-webhook-secret
```

### 9. Configure the webhook in GitHub

Go to your repository → **Settings** → **Webhooks** → **Add webhook**:

| Field | Value |
|---|---|
| Payload URL | `http://your-server-ip:9000/webhook` |
| Content type | `application/json` |
| Secret | same secret you passed to `--secret` |
| Events | Just the push event |

Now every `git push` triggers an automatic deployment.

---

## Manifest Reference

The `manifest.yml` file lives in your project root and defines how runway builds and runs your app.

```yaml
# Required: application name
app: my-service

# Optional: maximum time allowed for setup + build (seconds, default 600)
timeout: 600

# Optional: path to .env file on the server, relative to GITOPS_DIR
env_file: .env

# Optional: run before setup/build/start — failure aborts the deploy
pre_deploy:
  - ./scripts/pre-check.sh
  - echo "starting deploy $(date)"

# Optional: dependency installation commands (run before build)
setup:
  - npm install

# Optional: build commands
build:
  - npm run build

# Required: commands to start the service
start:
  - pm2 restart my-service || pm2 start dist/index.js --name my-service

# Optional: zero-downtime — poll URL before swapping symlink (old release stays live until healthy)
health_check:
  url: http://localhost:3000/health
  interval: 2     # seconds between polls (default 2)
  retries: 10     # max attempts before aborting (default 10)

# Optional: run after service is live — failure is logged, deploy is NOT reverted
post_deploy:
  - ./scripts/notify-slack.sh
  - ./scripts/run-smoke-tests.sh

# Optional: restrict which branches can trigger a deploy via webhook
# Exact match or trailing * wildcard. Empty = all branches allowed.
branches:
  - main
  - release/*

# Optional: email notifications (see Notifications section)
notify:
  email:
    to: ops@example.com
    from: runway@example.com
    smtp_host: smtp.gmail.com
    smtp_port: 587
```

### Field reference

| Field | Required | Description |
|---|---|---|
| `app` | yes | Application name (used in logs and notification emails) |
| `timeout` | no | Max seconds for setup+build phase. Default: `600` (10 min) |
| `env_file` | no | Path to `.env` file on server, relative to `GITOPS_DIR`. Guarded against path traversal. |
| `pre_deploy` | no | Commands run before setup/build/start. Failure aborts the deploy. |
| `setup` | no | Commands to install dependencies |
| `build` | no | Commands to build the project |
| `start` | yes | Commands to start/restart the service |
| `health_check.url` | no | HTTP endpoint to poll before symlink swap (zero-downtime) |
| `health_check.interval` | no | Seconds between health check polls. Default: `2` |
| `health_check.retries` | no | Max attempts before failing the deploy. Default: `10` |
| `post_deploy` | no | Commands run after service is live. Failure is **logged only** — deploy is not reverted. |
| `branches` | no | Branch patterns allowed to trigger webhook deploys. Empty = all branches. Supports `*` wildcard. |
| `notify.email.to` | no | Recipient email address |
| `notify.email.from` | no | Sender address (default: `runway@localhost`) |
| `notify.email.smtp_host` | no | SMTP server hostname |
| `notify.email.smtp_port` | no | SMTP port (default: `587`) |

### Runtime support

runway is language-agnostic. Any runtime works:

```yaml
# Node.js
setup:
  - npm install
build:
  - npm run build
start:
  - pm2 restart app || pm2 start dist/index.js

# Python
setup:
  - pip install -r requirements.txt
start:
  - systemctl restart my-python-app

# Go
setup:
  - go mod download
build:
  - go build -o bin/server ./cmd/server
start:
  - systemctl restart my-go-app

# Any custom command
start:
  - ./scripts/start.sh
```

### Deploy timeout

Setup and build commands share a single timeout (default 10 minutes, configurable via `timeout:`). If the timeout expires, all running commands are killed and the deploy fails cleanly — the partial release directory is removed, and your live app keeps running.

```yaml
timeout: 300  # 5 minutes — fail fast on slow builds
```

Start commands always get a separate fixed 2-minute window.

### Deploy hooks

`pre_deploy` runs before any setup, build, or start step. If any command exits non-zero, the deploy is aborted and the current symlink is not changed.

```yaml
pre_deploy:
  - ./scripts/check-disk-space.sh
  - ./scripts/backup-db.sh
```

`post_deploy` runs after the service is confirmed live (after `start` and, if configured, `health_check`). If a `post_deploy` command fails, it is logged as a warning — **the deploy is not reverted** since the service is already running.

```yaml
post_deploy:
  - curl -s http://localhost:3000/health
  - ./scripts/notify-slack.sh deployed $COMMIT
```

Both hooks run in the release directory with the same environment variables as the rest of the deploy.

### Zero-downtime deploy (health check)

Without `health_check:`, runway starts the service **then** atomically swaps the symlink. With `health_check:`, the order changes:

1. Start commands run on the new release directory
2. runway polls `health_check.url` until it returns HTTP 200
3. Only then is the symlink atomically swapped to the new release
4. Old release stays live the entire time

If the health check never passes, the deploy is aborted and the old release remains active — no partial update.

```yaml
start:
  - pm2 reload my-service --update-env

health_check:
  url: http://localhost:3000/health
  interval: 3    # poll every 3 seconds
  retries: 20    # give up after 20 × 3s = 60s
```

If `health_check.url` is empty (or the block is omitted), runway uses the classic "swap then start" behaviour.

### Branch-based deploy rules

`branches:` restricts which git branches are allowed to trigger a webhook deploy for this app. CLI deploys (`runway deploy <commit>`) always bypass branch rules.

```yaml
# Only deploy from main or any release/* branch
branches:
  - main
  - release/*
```

Patterns support a single trailing wildcard `*`:

| Pattern | Matches | Does not match |
|---|---|---|
| `main` | `main` | `main-v2`, `maintain` |
| `release/*` | `release/1.0`, `release/hotfix` | `main`, `feat/release/v2` |
| `feat/*` | `feat/login`, `feat/api` | `main`, `hotfix/foo` |

If `branches:` is omitted, **all branches** trigger a deploy.

---

## Multi-App Support

A single runway instance can manage multiple services using a `runway.yml` file in `GITOPS_DIR`.

### Setup

Create `runway.yml` in your runway working directory (e.g. `/opt/runway/runway.yml`):

```yaml
apps:
  - name: api
    repo: git@github.com:org/api.git
    base_dir: /opt/runway/api
    branches:
      - main
      - release/*

  - name: web
    repo: git@github.com:org/web.git
    base_dir: /opt/runway/web
    branches:
      - main

  - name: worker
    repo: git@github.com:org/worker.git
    base_dir: /opt/runway/worker
    # no branches: means all branches allowed
```

### How it works

When the webhook receives a push event and `runway.yml` exists, runway **fans out** — it triggers a parallel deploy for every app whose `branches:` list allows the pushed branch. Apps on non-matching branches are skipped silently.

Each app deploys independently:
- Its own `base_dir/releases/<commit>/` directory
- Its own `manifest.yml` (read from the cloned repo)
- Its own `deploy.log`
- Its own `history.json`

Without `runway.yml`, runway operates in **single-app mode** using the `GITOPS_DIR` and `GITOPS_REPO` environment variables as before.

### `runway.yml` field reference

| Field | Required | Description |
|---|---|---|
| `apps[].name` | yes | Human-readable app identifier (used in logs) |
| `apps[].repo` | yes | Git repository URL |
| `apps[].base_dir` | yes | Working directory for this app. Must be unique across all apps. |
| `apps[].branches` | no | Branch patterns allowed to deploy. Same `*` wildcard semantics as `manifest.yml`. Empty = all branches. |

### Directory layout (multi-app)

```
/opt/runway/
├── runway.yml              ← multi-app config
│
├── api/
│   ├── .env                ← secrets for api
│   ├── history.json
│   └── releases/
│       └── abc123/
│           └── deploy.log
│
├── web/
│   ├── .env                ← secrets for web
│   ├── history.json
│   └── releases/
│       └── abc123/
│           └── deploy.log
│
└── worker/
    ├── .env
    ├── history.json
    └── releases/
```

The `GITOPS_GIT_TOKEN` environment variable is shared across all apps (set once in `/etc/runway/env`). Each app has its own `.env` file for app-specific secrets.

---

## CLI Reference

```
runway <command> [flags]
```

### Global flags

| Flag | Description |
|---|---|
| `--no-color` | Disable all ANSI color output. Also respected via `NO_COLOR` env var. |

### Commands at a glance

| Command | Description |
|---|---|
| `runway init` | Interactively create manifest.yml and directory structure |
| `runway doctor` | Check setup and diagnose problems |
| `runway deploy <commit>` | Deploy a specific commit |
| `runway rollback <commit>` | Roll back to a previously deployed commit |
| `runway status` | Show current deployment status and recent history |
| `runway releases` | List all stored releases |
| `runway history` | Show full deployment history |
| `runway listen` | Start the webhook listener (HTTP server) |
| `runway log <commit>` | Print the deploy log for a commit |
| `runway version` | Print version information |

---

### `runway init`

```bash
runway init
```

Interactively creates your deployment setup:

1. Detects your runtime (Node.js, Python, Go, Ruby) from project files
2. Prompts for app name, git repo URL, deploy directory, webhook port, and commands
3. Writes `manifest.yml` and `releases/` directory
4. Creates an empty `.env` file (mode `0600`) if one doesn't exist
5. Prints the exact next steps to complete setup

```
 runway init

 → Detected: Node.js

  App name [my-app]:
  Git repo URL []:  git@github.com:org/my-app.git
  Deploy directory [/opt/runway]:
  Webhook port [9000]:
  Setup commands [npm install]:
  Build commands [npm run build]:
  Start commands [pm2 restart app || pm2 start dist/index.js --name app]:

 ✓ /opt/runway/releases created
 ✓ manifest.yml created (/opt/runway/manifest.yml)
 ✓ .env created (/opt/runway/.env)

 Next steps:

   1. Add your secrets to /opt/runway/.env
   2. Set environment variable: export GITOPS_REPO=git@github.com:org/my-app.git
   3. Add webhook in GitHub/GitLab:  http://your-server:9000/webhook
   4. Run: runway listen --port 9000 --secret <your-secret>
```

---

### `runway doctor`

```bash
runway doctor
```

Runs 8 checks against your server setup and reports what's wrong:

1. `git` binary is installed
2. `GITOPS_REPO` is set
3. `GITOPS_DIR` exists
4. `GITOPS_DIR` is writable
5. `releases/` directory exists
6. `manifest.yml` is valid
7. `.env` file is readable (if configured in manifest)
8. No stale lock file
9. Git authentication is available (HTTPS token or SSH key)

```
 Checking runway setup...

 ✓ git is installed (git version 2.44.0)
 ✓ GITOPS_REPO is set (git@github.com:org/my-app.git)
 ✓ GITOPS_DIR exists (/opt/runway)
 ✓ GITOPS_DIR is writable (/opt/runway)
 ✓ releases/ directory exists (/opt/runway/releases)
 ✓ manifest.yml is valid (app: my-app)
 ✓ no stale lock file
 ✗ git auth — no GITOPS_GIT_TOKEN or SSH key found
      → Fix: set GITOPS_GIT_TOKEN=<token> or add an SSH key (~/.ssh/id_ed25519)

 ⚠ 1 issue(s) found — fix the above and re-run 'runway doctor'
```

Returns exit code `1` if any issues are found.

---

### `runway deploy`

```bash
runway deploy <commit>
runway deploy --dry-run <commit>
```

Deploys the specified git commit SHA. Runs the full sequence: clone → parse manifest → load env → setup → build → swap symlink → start.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--dry-run` | false | Validate config and print what would run, without executing |

**Env vars required:** `GITOPS_REPO`, `GITOPS_DIR`

#### Dry-run mode

`--dry-run` validates your entire configuration without cloning or running any commands:

```bash
runway deploy --dry-run abc123def456
```

```
 DRY RUN — no changes will be made

 ✓ git is installed
 ✓ GITOPS_REPO is set (git@github.com:org/my-app.git)
 ✓ GITOPS_DIR exists (/opt/runway)
 ✓ releases/ directory exists (/opt/runway/releases)
 ✓ manifest.yml valid (app: my-app)
 ✓ .env file readable (/opt/runway/.env)

 Would run:
    setup:   npm install
    build:   npm run build
    start:   pm2 restart app || pm2 start dist/index.js --name app

    → timeout: 600s

 → run without --dry-run to deploy commit abc123de
```

#### Auto-rollback

If the `start` commands fail after the symlink has been switched, runway automatically rolls back to the previous release:

```
 ✗ deploy failed: start failed — automatically rolled back to 345678ab
```

---

### `runway rollback`

```bash
runway rollback <commit>
```

Switches the active release to a previously deployed commit. **No rebuild.** Swaps the symlink and restarts the service. Takes ~1 second.

```bash
# See available commits
runway releases

# Roll back
runway rollback 345678ab
```

---

### `runway status`

```bash
runway status
```

```
─────────────────────────────────────
 runway status
─────────────────────────────────────
 active:   abc123def456

 recent deployments:
 commit          time                  status
 ──────────────────────────────────────────────────────
 abc123def456    2026-03-09 12:00:05   running
 345678987t      2026-03-08 18:30:00   previous
 98sd76asdf      2026-03-07 10:15:00   rolled_back
─────────────────────────────────────
```

---

### `runway releases`

```bash
runway releases
```

Lists all release directories currently on disk. The active one is marked.

```
commit          active
──────────────
abc123def456    ← current
345678987tg
98sd76asdf
```

---

### `runway history`

```bash
runway history
runway history --limit 5
runway history --status failed
```

Shows the full deployment history from `history.json`.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--limit N` | `0` (all) | Show only the most recent N entries |
| `--status S` | (all) | Filter by status: `running`, `failed`, `previous`, `rolled_back` |

```
 COMMIT          TIME                  STATUS          BY
 ────────────────────────────────────────────────────────────────────────
 abc123def456    2026-03-09 12:00:05   running         webhook
 345678987t      2026-03-08 18:30:00   previous        cli
 98sd76asdf      2026-03-07 10:15:00   rolled_back     webhook
 abcdef123456    2026-03-06 09:00:00   failed          webhook
```

---

### `runway listen`

```bash
runway listen --port 9000 --secret your-webhook-secret
runway listen --port 9000 --secret your-webhook-secret --log-format json
runway listen --port 9000 --secret your-webhook-secret --webhook-rate-limit 10
```

Starts the HTTP webhook listener. Runs as a long-lived process — use systemd to keep it alive (see below).

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--port` | `9000` | TCP port to listen on |
| `--secret` | — | HMAC-SHA256 signing secret (required, or set `GITOPS_WEBHOOK_SECRET`) |
| `--log-format` | `text` | Event log format: `text` or `json` |
| `--webhook-rate-limit` | `5` | Max webhook requests per minute. `0` disables rate limiting. |

The secret can also be set via the `GITOPS_WEBHOOK_SECRET` environment variable.

**Graceful shutdown:** `runway listen` handles `SIGINT` and `SIGTERM`. On signal, it stops accepting new connections and waits up to 10 seconds for HTTP connections to drain, then waits for any in-flight deploy to finish before exiting.

#### JSON log format

Use `--log-format json` to emit structured newline-delimited JSON to stderr, suitable for Loki, Splunk, or any log aggregator:

```bash
runway listen --port 9000 --secret $SECRET --log-format json 2>> /var/log/runway/events.log
```

```json
{"time":"2026-03-09T12:00:04Z","level":"info","event":"deploy.start","commit":"abc123def456","triggered":"webhook"}
{"time":"2026-03-09T12:00:09Z","level":"info","event":"deploy.success","commit":"abc123def456","triggered":"webhook","duration_s":5.1}
{"time":"2026-03-09T12:00:14Z","level":"error","event":"deploy.failed","commit":"789abcdef012","triggered":"webhook","duration_s":3.2,"error":"build: exit status 1"}
```

#### Rate limiting

By default, the `/webhook` endpoint accepts at most **5 requests per minute** (token bucket algorithm). Requests over the limit receive:

```
HTTP 429 Too Many Requests
Retry-After: 12
```

To increase the limit for busy repos, or disable it entirely:

```bash
runway listen --webhook-rate-limit 30   # 30/min
runway listen --webhook-rate-limit 0    # unlimited
```

---

### `runway log`

```bash
runway log <commit>
```

Prints the full deploy log (stdout + stderr of all commands) for a specific commit. Useful for debugging failed deployments.

```bash
runway log abc123def456
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GITOPS_REPO` | for deploy/listen | Git repository URL (SSH or HTTPS) |
| `GITOPS_DIR` | no | Working directory (default: `/opt/runway`) |
| `GITOPS_GIT_TOKEN` | no | Git HTTPS auth token (never logged or written to disk) |
| `GITOPS_WEBHOOK_SECRET` | for listen | Webhook HMAC secret (alternative to `--secret` flag) |
| `RUNWAY_SMTP_PASSWORD` | for notifications | SMTP password for email notifications |
| `NO_COLOR` | no | Set to any value to disable ANSI color output |

---

## Exit Codes

runway uses distinct exit codes for scripting and CI integration:

| Code | Constant | Meaning |
|---|---|---|
| `0` | `OK` | Success |
| `1` | `GeneralError` | Unknown or unclassified error |
| `2` | `LockHeld` | Another deploy is already in progress |
| `3` | `BuildFailed` | A setup or build command exited non-zero |
| `4` | `StartFailed` | Service failed to start (auto-rollback may have triggered) |
| `5` | `GitError` | Clone or checkout failed |
| `6` | `ManifestError` | `manifest.yml` is missing or invalid |
| `7` | `NotFound` | Rollback target release does not exist |

Example usage in scripts:

```bash
runway deploy abc123 || {
  code=$?
  case $code in
    2) echo "Deploy already running — try again shortly" ;;
    3) echo "Build failed — check runway log abc123" ;;
    4) echo "Service failed to start — auto-rollback triggered" ;;
    5) echo "Git error — check GITOPS_REPO and credentials" ;;
  esac
  exit $code
}
```

---

## Notifications

runway can send email notifications after every deploy. Configure them in `manifest.yml`:

```yaml
notify:
  email:
    to: ops@example.com
    from: runway@example.com
    smtp_host: smtp.gmail.com
    smtp_port: 587
```

Set the SMTP password as an environment variable on the server — never in `manifest.yml`:

```bash
export RUNWAY_SMTP_PASSWORD=your-smtp-password
```

Or in `/etc/runway/env` (see systemd section).

**Email subjects:**

| Event | Subject |
|---|---|
| Success | `✓ runway: deployed abc123def456 to my-app` |
| Start failure + auto-rollback | `⚠ runway: auto-rolled back my-app to 345678ab` |
| Deploy failure | `✗ runway: deploy failed on my-app` |

Failure emails include the last 20 lines of `deploy.log` to help diagnose problems without SSH access.

---

## Security

- **Webhook signatures** — every request is verified with HMAC-SHA256 (`X-Hub-Signature-256`). Requests without a valid signature are rejected with HTTP 401.
- **Rate limiting** — the `/webhook` endpoint is rate-limited (default 5/min) to prevent abuse. Returns HTTP 429 with `Retry-After` on excess requests.
- **Body size limit** — webhook payloads are limited to 5MB to prevent memory exhaustion.
- **Path traversal guard** — `env_file` paths in `manifest.yml` are resolved through `filepath.EvalSymlinks` and must fall within `GITOPS_DIR`. Traversal attempts (e.g. `../../etc/passwd`) are rejected.
- **Command injection audit** — during every deploy, runway scans manifest commands for suspicious shell patterns (`$(`, `` ` ``, `eval`, `curl`, `wget`, `nc`, `bash -c`, etc.) and logs warnings. Deploys are not aborted — warnings are advisory so operators can verify intent.
- **No world-readable secrets** — `.env` is created with mode `0600`. The `GITOPS_GIT_TOKEN` and `RUNWAY_SMTP_PASSWORD` are never written to disk or logged.
- **Limited deploy user** — the `deploy` user has no sudo access; it only owns `/opt/runway`.
- **Build failure safety** — if any step before the symlink switch fails, the partial release directory is deleted. Your live app keeps running.
- **Concurrent deploy lock** — a file lock at `/tmp/runway.lock` prevents overlapping deploys. Stale locks (from crashed processes) are detected and reported.
- **Graceful shutdown** — in-flight deploys are always allowed to complete before runway exits.

---

## Deployment History

runway keeps a `history.json` file in `GITOPS_DIR` tracking the last 15 deployments:

```json
{
  "current": "abc123def456",
  "deployments": [
    {
      "commit": "abc123def456",
      "time": "2026-03-09T12:00:05Z",
      "status": "running",
      "triggered": "webhook"
    },
    {
      "commit": "345678987tg",
      "time": "2026-03-08T18:30:00Z",
      "status": "previous",
      "triggered": "cli"
    }
  ]
}
```

| Status | Meaning |
|---|---|
| `running` | currently active release |
| `previous` | previously active, superseded by a newer deploy |
| `failed` | build or start step failed |
| `rolled_back` | this commit was the target of a rollback |

A `history.json.bak` backup is written before every update. If `history.json` becomes corrupt, runway automatically recovers from the backup on the next operation and logs a warning.

---

## Server Directory Layout

```
/opt/runway/
├── history.json          ← managed by runway (last 15 deployments)
├── history.json.bak      ← automatic backup, used for recovery
├── .env                  ← your secrets, never in git (mode 0600)
│
├── releases/
│   ├── abc123def456/
│   │   ├── manifest.yml  ← cloned from git
│   │   └── deploy.log    ← full build output for this release
│   ├── 345678987tg/
│   │   ├── manifest.yml
│   │   └── deploy.log
│   └── 98sd76as/
│       ├── manifest.yml
│       └── deploy.log
│
└── current -> releases/abc123def456   ← active symlink (atomic swap)
```

runway keeps the last **15 releases** on disk. Older ones are cleaned up automatically after each successful deploy. The active release is never deleted.

---

## Production Setup (systemd)

Running runway as a systemd service keeps the webhook listener alive across reboots and auto-restarts it on failure.

### 1. Create a dedicated deploy user

```bash
sudo useradd --system --shell /bin/bash --home /opt/runway deploy
sudo mkdir -p /opt/runway
sudo chown deploy:deploy /opt/runway
```

### 2. Create the environment file

```bash
sudo mkdir -p /etc/runway
sudo tee /etc/runway/env > /dev/null <<EOF
GITOPS_REPO=git@github.com:yourorg/your-repo.git
GITOPS_DIR=/opt/runway
GITOPS_WEBHOOK_SECRET=your-webhook-secret
GITOPS_GIT_TOKEN=
RUNWAY_SMTP_PASSWORD=
EOF
sudo chmod 600 /etc/runway/env
```

### 3. Install the systemd service

```bash
sudo cp systemd/runway.service /etc/systemd/system/runway.service
sudo systemctl daemon-reload
sudo systemctl enable runway
sudo systemctl start runway
```

### 4. Check status

```bash
sudo systemctl status runway
sudo journalctl -u runway -f
```

### Systemd service file

```ini
[Unit]
Description=Runway Webhook Listener
After=network.target

[Service]
Type=simple
User=deploy
EnvironmentFile=/etc/runway/env
ExecStart=/usr/local/bin/runway listen \
  --port 9000 \
  --secret ${GITOPS_WEBHOOK_SECRET} \
  --log-format json \
  --webhook-rate-limit 5
Restart=on-failure
RestartSec=5s
MemoryMax=64M
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

> With `--log-format json`, structured events go to stderr and can be picked up by `journald` or forwarded to a log aggregator via `StandardError=append:/var/log/runway/events.log`.

---

## Git Authentication

### Option A — SSH Deploy Key (recommended)

1. Generate a key pair on the server:

```bash
sudo -u deploy ssh-keygen -t ed25519 -C "runway-deploy" -f /home/deploy/.ssh/runway_deploy
```

2. Add the public key as a **Deploy Key** in your GitHub repo:
   `Settings → Deploy keys → Add deploy key` (read-only access)

3. Configure SSH for the deploy user:

```bash
# /home/deploy/.ssh/config
Host github.com
  IdentityFile ~/.ssh/runway_deploy
  StrictHostKeyChecking no
```

4. Use an SSH URL in `GITOPS_REPO`:

```
GITOPS_REPO=git@github.com:yourorg/your-repo.git
```

### Option B — HTTPS Token

```bash
# /etc/runway/env
GITOPS_REPO=https://github.com/yourorg/your-repo.git
GITOPS_GIT_TOKEN=ghp_your_personal_access_token
```

runway injects the token into the clone URL automatically. The token is never written to disk or logged.

---

## Performance

| Metric | Value |
|---|---|
| Binary size | ~3–5 MB |
| Memory (idle) | ~5–8 MB |
| Deploy time | 5–30s (depends on build) |
| Rollback time | ~1 second |
| Dependencies on server | Git only |

---

## Requirements

| Component | Requirement |
|---|---|
| OS | Linux (amd64 or arm64) |
| RAM | 512 MB minimum |
| Disk | 5 GB minimum |
| Git | must be installed |
| PM2 | optional, for Node.js apps |

macOS binaries are provided for local testing only. Production use is Linux.

---

## Contributing

Contributions are welcome.

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Make your changes
4. Run tests: `make test`
5. Run vet: `make vet`
6. Open a pull request

Please keep pull requests focused — one feature or fix per PR.

### Development setup

```bash
git clone https://github.com/Reeteshrajesh/runway.git
cd runway
go build ./...     # verify it compiles
make test          # run tests
make build         # build binary
./runway version   # smoke test
```

---

## Roadmap

- [x] Zero-downtime deployments (health check before symlink swap) ✓
- [x] Multi-app support from a single runway instance ✓
- [x] Branch-based deployment rules ✓
- [x] Deploy hooks (`pre_deploy` / `post_deploy`) ✓
- [ ] Deployment dashboard (web UI)
- [ ] Homebrew tap
- [ ] Remote deploy (`runway deploy --target user@host commit`)
- [ ] Health check dashboard / status page

---

## License

[MIT](LICENSE) — Copyright © 2026 Reetesh Kumar
