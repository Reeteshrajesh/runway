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
- **Manifest-driven** — define setup, build, and start commands in `manifest.yml`
- **Webhook listener** — receives GitHub/GitLab push events with HMAC verification
- **Concurrent deploy protection** — file lock prevents overlapping deployments
- **Build failure safety** — broken builds are deleted before going live
- **Per-deploy logs** — full stdout/stderr saved to `releases/<commit>/deploy.log`
- **Environment variable support** — inject secrets via `.env` file, never stored in git
- **Deployment history** — tracks last 15 deployments in `history.json`
- **Single binary** — copy one file to your server, done

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

### Option 2 — Build from source

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

### 1. Set up the server directory

```bash
sudo mkdir -p /opt/runway/releases
sudo chown -R deploy:deploy /opt/runway
```

> Replace `deploy` with the user that will run runway on your server.

### 2. Create a `manifest.yml` in your project root

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

### 3. Create a `.env` file on the server (not in git)

```bash
# /opt/runway/.env
DATABASE_URL=postgres://localhost/mydb
API_KEY=your-secret-key
NODE_ENV=production
```

### 4. Set environment variables for runway

```bash
export GITOPS_REPO=git@github.com:yourorg/your-repo.git
export GITOPS_DIR=/opt/runway
export GITOPS_WEBHOOK_SECRET=your-webhook-secret
```

> For permanent setup, add these to `/etc/runway/env` (see systemd section below).

### 5. Test a manual deploy

```bash
runway deploy <commit-sha>
```

### 6. Start the webhook listener

```bash
runway listen --port 9000 --secret your-webhook-secret
```

### 7. Configure the webhook in GitHub

Go to your repository → **Settings** → **Webhooks** → **Add webhook**:

| Field | Value |
|---|---|
| Payload URL | `http://your-server-ip:9000/webhook` |
| Content type | `application/json` |
| Secret | same secret you passed to `--secret` |
| Events | Just the push event |

Now every `git push` to your repo will trigger an automatic deployment.

---

## Manifest Reference

The `manifest.yml` file lives in your project root and defines how runway builds and runs your app.

```yaml
# Required: application name
app: my-service

# Optional: path to .env file on the server (relative to GITOPS_DIR)
env_file: .env

# Optional: dependency installation commands (run before build)
setup:
  - npm install

# Optional: build commands
build:
  - npm run build

# Required: commands to start the service
start:
  - pm2 restart my-service || pm2 start dist/index.js --name my-service
```

### Field reference

| Field | Required | Description |
|---|---|---|
| `app` | yes | Application name (used in logs) |
| `env_file` | no | Path to `.env` file on server, relative to `GITOPS_DIR` |
| `setup` | no | Commands to install dependencies |
| `build` | no | Commands to build the project |
| `start` | yes | Commands to start/restart the service |

### Runtime support

runway is language-agnostic. Any runtime works:

```yaml
# Node.js
start:
  - pm2 restart app || pm2 start dist/index.js

# Python
start:
  - systemctl restart my-python-app

# Go
start:
  - systemctl restart my-go-app

# Any custom command
start:
  - ./scripts/start.sh
```

---

## CLI Reference

```
runway <command> [flags]
```

| Command | Description |
|---|---|
| `runway deploy <commit>` | Deploy a specific commit |
| `runway rollback <commit>` | Roll back to a previously deployed commit |
| `runway status` | Show current deployment status and recent history |
| `runway releases` | List all stored releases |
| `runway listen` | Start the webhook listener |
| `runway log <commit>` | Print the deploy log for a specific commit |
| `runway version` | Print version information |

### `runway deploy`

```bash
runway deploy abc123def456
```

Pulls the specified commit, runs manifest steps, updates the active release.

**Flags:** none
**Env vars required:** `GITOPS_REPO`, `GITOPS_DIR`

---

### `runway rollback`

```bash
runway rollback abc123def456
```

Switches the active release to a previously deployed commit. No rebuild — just a symlink swap and service restart. Takes ~1 second.

```bash
# See available commits to roll back to
runway releases
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

```
commit          active
──────────────
abc123def456    ← current
345678987t
98sd76asdf
```

---

### `runway listen`

```bash
runway listen --port 9000 --secret your-webhook-secret
```

Starts the HTTP webhook listener. Runs as a long-lived process — use systemd to keep it alive (see below).

| Flag | Default | Description |
|---|---|---|
| `--port` | `9000` | TCP port to listen on |
| `--secret` | — | HMAC signing secret (required) |

The secret can also be set via the `GITOPS_WEBHOOK_SECRET` environment variable.

---

### `runway log`

```bash
runway log abc123def456
```

Prints the full deploy log (stdout + stderr of all commands) for a specific commit. Useful for debugging failed deployments.

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GITOPS_REPO` | for deploy/listen | Git repository URL (SSH or HTTPS) |
| `GITOPS_DIR` | no | Working directory (default: `/opt/runway`) |
| `GITOPS_GIT_TOKEN` | no | Git HTTPS auth token |
| `GITOPS_WEBHOOK_SECRET` | for listen | Webhook HMAC secret (alternative to `--secret`) |

---

## Server Directory Layout

```
/opt/runway/
├── manifest.yml          ← read from cloned repo, not here
├── history.json          ← managed by runway
├── .env                  ← your secrets, never in git
│
├── releases/
│   ├── abc123def456/
│   │   └── deploy.log    ← full build output
│   ├── 345678987tg/
│   │   └── deploy.log
│   └── 98sd76as/
│       └── deploy.log
│
└── current -> releases/abc123def456   ← active symlink
```

runway keeps the last **15 releases** on disk. Older ones are cleaned up automatically. The active release is never deleted.

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
ExecStart=/usr/local/bin/runway listen --port 9000 --secret ${GITOPS_WEBHOOK_SECRET}
Restart=on-failure
RestartSec=5s
MemoryMax=64M
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

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

## Security

- **Webhook signatures** — every incoming webhook is verified with HMAC-SHA256 (`X-Hub-Signature-256`). Requests without a valid signature are rejected with HTTP 401.
- **No world-readable secrets** — `.env` file should be `chmod 600`, owned by the deploy user.
- **Limited deploy user** — the `deploy` user has no sudo access. It only owns `/opt/runway`.
- **Build failure safety** — if any build step fails, the partial release directory is deleted and the current symlink is never updated. Your live app keeps running.
- **Concurrent deploy lock** — a file lock at `/tmp/runway.lock` prevents two deploys from running at the same time.
- **Body size limit** — the webhook server limits request bodies to 5MB to prevent memory exhaustion.

---

## Deployment History

runway keeps a `history.json` file in `GITOPS_DIR` with the last 15 deployments:

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
| `previous` | previously active, superseded |
| `failed` | build or start step failed |
| `rolled_back` | this commit was rolled back to |

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

- [ ] Zero-downtime deployments (blue/green symlink swap)
- [ ] Health check validation before symlink switch
- [ ] Multi-app support from a single runway instance
- [ ] Branch-based deployment rules (e.g. `main` → production)
- [ ] Deployment dashboard (web UI)
- [ ] Homebrew tap

---

## License

[MIT](LICENSE) — Copyright © 2026 Reetesh Kumar
