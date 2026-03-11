# Contributing to runway

Thank you for considering contributing to runway. Contributions of all kinds are welcome — bug fixes, new features, documentation improvements, and test coverage.

---

## Ground rules

- **One PR per concern** — keep pull requests focused. A bug fix and a new feature should be two separate PRs.
- **Zero external dependencies** — runway uses only the Go standard library. Do not introduce third-party packages.
- **Tests required** — every new feature or bug fix must include tests. PRs without tests will not be merged.
- **No `os.Exit` in packages** — only `cmd/runway/main.go` calls `os.Exit`. All other code returns errors.

---

## Development setup

Requires Go 1.22 or later.

```bash
git clone https://github.com/Reeteshrajesh/runway.git
cd runway

go build ./...      # verify it compiles
make test           # run all tests with race detector
make vet            # run go vet
make build          # build binary to ./runway
./runway version    # smoke test
```

### Make targets

| Target | Description |
|---|---|
| `make build` | Build binary for the current platform |
| `make test` | Run all tests with race detector |
| `make vet` | Run `go vet` |
| `make lint` | Run vet (add golangci-lint if desired) |
| `make install` | Build and install to `/usr/local/bin` |
| `make release` | Cross-compile all platform binaries to `dist/` |
| `make clean` | Remove build artifacts |

---

## Project structure

```
runway/
├── cmd/runway/          # main package — entry point only, no logic
├── internal/
│   ├── cli/             # command-line commands (deploy, rollback, listen, …)
│   ├── color/           # ANSI terminal colour helpers
│   ├── engine/          # core deploy/rollback logic, lock, git clone
│   ├── envloader/       # .env file parser
│   ├── logger/          # deploy log + structured event log
│   ├── manifest/        # manifest.yml parser and auditor
│   ├── notify/          # email notifications via net/smtp
│   ├── release/         # release directory manager + history.json
│   └── webhook/         # HTTP webhook server + rate limiter
├── systemd/             # example systemd service file
├── .github/
│   ├── workflows/       # CI and release workflows
│   └── ISSUE_TEMPLATE/  # bug report and feature request templates
├── Makefile
└── README.md
```

---

## Adding a new command

1. Create `internal/cli/<command>.go` with a `run<Command>(args []string) error` function.
2. Register it in `internal/cli/cli.go` `Run()` switch statement.
3. Update `printUsage()` in `internal/cli/cli.go`.
4. Add a section to `README.md` under **CLI Reference**.
5. Write tests in `internal/cli/<command>_test.go`.

---

## Adding a manifest field

1. Add the field to the `Manifest` struct in `internal/manifest/parser.go`.
2. Handle the new key in the `ParseFile` scanner loop.
3. Update `Validate()` if the field is required.
4. Add a test case to `internal/manifest/parser_test.go`.
5. Document the field in the **Manifest Reference** table in `README.md`.

---

## Running tests

```bash
# All tests
make test

# Single package
go test -race ./internal/engine/...

# Verbose output
go test -race -v ./internal/manifest/...

# Specific test
go test -race -run TestDeploy_ContextTimeout ./internal/engine/...
```

---

## Commit message style

Use the imperative mood in the subject line. Keep it under 72 characters.

```
add --dry-run flag to runway deploy
fix stale lock detection on Linux
update manifest parser to support timeout field
```

No ticket numbers or emoji required.

---

## Pull request checklist

Before opening a PR:

- [ ] `make test` passes with no failures
- [ ] `make vet` passes with no warnings
- [ ] `gofmt -l .` reports no unformatted files
- [ ] New behaviour is covered by tests
- [ ] `README.md` updated if a user-visible feature was added or changed
- [ ] No new external dependencies introduced

---

## Reporting bugs

Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md). Always include:

- `runway version` output
- OS and architecture
- The deploy log (`runway log <commit>`)
- Steps to reproduce

---

## License

By contributing you agree that your contributions will be licensed under the [MIT License](../LICENSE).
