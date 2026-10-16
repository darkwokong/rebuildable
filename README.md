# Rebuildable

Rebuildable audits whether a repository can be cloned and run on a clean Linux machine.

The CLI checks the same reproducibility rules a reviewer cares about: Git history, dependency lockfiles, pinned runtimes, containers, tests, documentation, environment variables, and machine-specific paths.

## Setup

Rebuildable is written in Go. Install Go 1.27 or newer, then download the locked module dependencies:

```sh
go mod download
```

Docker is only needed for the later verification command. The audit command and the default test suite do not require Docker, network access, or API keys.

## Run

Audit the current repository:

```sh
go run ./cmd/rebuildable audit .
```

Session 1 implements the repository loader, ecosystem detection, and the first three checks. The remaining checks are reported as not implemented until the later build sessions.

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
