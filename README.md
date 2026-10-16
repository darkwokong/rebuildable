# Rebuildable

Rebuildable tells you whether a stranger can clone and run a repository on a clean Linux machine—and generates the missing reproducibility files.

The CLI checks the same reproducibility rules a reviewer cares about: Git history, dependency lockfiles, pinned runtimes, containers, tests, documentation, environment variables, and machine-specific paths. Static scans use tracked project text and skip vendored dependencies, binaries, and files over 1 MB.

## Setup

Rebuildable is written in Go. Install Go 1.27 or newer. Dependencies are vendored so the test suite can run without network access.

```sh
go version
```

Docker is only needed for the verification command. The audit, fix, and default test suite do not require API keys. Dependencies are vendored for offline builds.

## Run

Audit the current repository:

```sh
go run ./cmd/rebuildable audit .
```

Audit a local path or shallow-clone a Git URL:

```sh
go run ./cmd/rebuildable audit ./path/to/repository
go run ./cmd/rebuildable audit https://example.com/owner/project.git
```

Write machine-readable reports and enforce a CI threshold:

```sh
go run ./cmd/rebuildable audit . --json
go run ./cmd/rebuildable audit . --md audit.md
go run ./cmd/rebuildable audit . --min-score 8
```

Generate missing reproducibility files. The default is a dry-run diff:

```sh
go run ./cmd/rebuildable fix .
go run ./cmd/rebuildable fix . --write
go run ./cmd/rebuildable fix . --write --force
go run ./cmd/rebuildable fix . --only dockerfile,env,readme,test,pins
```

Prove the repository in Docker:

```sh
go run ./cmd/rebuildable verify . --timeout 10m --keep-logs
```

Verification builds with `docker build --no-cache`, builds a dedicated test stage when the Dockerfile provides one, starts the application image, probes an explicitly exposed port, runs the test command in an isolated container, and cleans up its image and containers. Logs are retained on failure and can be retained on success with `--keep-logs`.

The audit implements all eight static checks. Warnings and failures do not earn points; only full passes count toward the score.

## Test

Run the offline test suite:

```sh
make test
```

Run static analysis:

```sh
make lint
```

Run the reproducibility demo:

```sh
make demo
```

Docker-backed integration checks are kept separate:

```sh
make test-integration
```

## Development status

The intended end state is:

```text
rebuildable audit [path|git-url]
rebuildable fix [path] [--write]
rebuildable verify [path]
```

The audit, fix, and verify flows will remain usable without any paid service or API key.

## GitHub Action

The included workflow audits every push and pull request with a required `8/8` threshold. It uses the same local CLI and does not need a secret.

## Exit codes

- `0` — the audit completed and met `--min-score` (or no threshold was supplied).
- `1` — the audit completed but scored below `--min-score`.
- `2` — the CLI could not load the repository, clone the Git URL, or write a report.
