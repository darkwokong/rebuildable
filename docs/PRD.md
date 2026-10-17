# Rebuildable product requirements

## Product

Rebuildable is a Go CLI that tells a developer whether another person can
clone and run a repository on a clean Linux machine. It audits reproducibility
without Docker, can generate safe missing project files, and can prove a
trusted repository in a disposable Docker workflow.

## Users

- **Builder:** wants to catch rebuild failures before submitting a project.
- **Reviewer:** wants a fast, evidence-backed score before spending time on a
  submission.
- **Maintainer:** wants a CI threshold that fails when reproducibility regresses.

## Commands

```text
rebuildable audit [path|git-url] [--json] [--md report.md] [--min-score N]
rebuildable fix [path] [--write] [--force] [--only ...] [--ecosystem ...]
rebuildable verify [path] --trust [--timeout 10m] [--port 8080] [--keep-logs]
```

## Static audit acceptance criteria

The audit runs eight deterministic checks and counts only full passes:

1. Git repository with at least three reachable commits.
2. Ecosystem-specific dependency lockfile.
3. Exact runtime pin where the supported ecosystem can provide one.
4. Dockerfile with `FROM` and a default run instruction, plus an optional
   `.dockerignore` warning.
5. A recognized test command.
6. README Setup, Run, and Test sections.
7. Environment-variable coverage, tracked `.env` detection, and a focused
   obvious-secret scan.
8. Machine-specific absolute paths in scanned text.

Every warning or failure includes evidence and a next action when one can be
identified. Git-tracked files are the audit source for repositories with Git;
local non-Git paths are scanned as working trees but cannot pass the Git check.
Binary files, symlinks, dependency trees, and files over 1 MiB are excluded.

## Fix acceptance criteria

- Dry-run is the default and emits repository-relative unified diffs.
- `--write` creates missing generated files.
- Existing differing files require `--force`.
- Writes reject symlink targets and paths outside the workspace.
- Generators are deterministic and idempotent.
- Lockfiles and secrets are not fabricated or copied by the tool; unresolved
  manual work remains visible in the next audit.

## Verify acceptance criteria

Verification is explicitly trusted code execution. It requires `--trust`,
builds with Docker without cache, uses disposable named resources, applies
runtime limits, binds HTTP probes to localhost, runs tests in a dedicated stage
when available, records logs on failure, and removes owned containers/images.

The default audit score is static evidence. A successful verify is the dynamic
proof that completes the rebuild story.

## Non-goals for the production MVP

- Vulnerability/CVE scanning.
- Full secret scanning.
- Windows containers or Kubernetes manifests.
- Monorepo traversal.
- Rust, Java, Ruby, and .NET ecosystem support.
- Web dashboard or paid API dependency.
