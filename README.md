# Rebuildable

Rebuildable audits whether a repository can be cloned and run on a clean Linux machine.

The CLI checks the same reproducibility rules a reviewer cares about: Git history, dependency lockfiles, pinned runtimes, containers, tests, documentation, environment variables, and machine-specific paths. Static scans use tracked project text and skip vendored dependencies, binaries, and files over 1 MB.

## Setup

Rebuildable is written in Go. Install Go 1.27 or newer. Dependencies are vendored so the test suite can run without network access.

```sh
go version
```

Docker is only needed for the later verification command. The audit command and the default test suite do not require Docker, network access, or API keys.

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

The audit currently implements all eight static checks. Docker build/run verification is reserved for the `verify` command in Session 3.

## Test

Run the offline test suite:

```sh
make test
```

Run static analysis:

```sh
make lint
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

## Exit codes

- `0` — the audit completed and met `--min-score` (or no threshold was supplied).
- `1` — the audit completed but scored below `--min-score`.
- `2` — the CLI could not load the repository, clone the Git URL, or write a report.
