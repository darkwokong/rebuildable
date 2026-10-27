package checks

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

var inlineComment = regexp.MustCompile(`\s#`)

type lockfileCheck struct{}

func (lockfileCheck) ID() string   { return "2" }
func (lockfileCheck) Name() string { return "Dependency lockfile" }

func (lockfileCheck) Run(_ context.Context, r *repo.Repo) Result {
	result := Result{CheckID: "2", Fixable: false}
	if len(r.Ecosystems) == 0 {
		result.Status = StatusSkip
		result.Findings = []Finding{{
			Message: "no supported dependency manifest detected",
			Hint:    "add go.mod, package.json, pyproject.toml, or requirements.txt",
		}}
		return result
	}

	statuses := make([]Status, 0, len(r.Ecosystems))
	for _, ecosystem := range r.Ecosystems {
		status, finding := lockfileForEcosystem(r, ecosystem)
		statuses = append(statuses, status)
		result.Findings = append(result.Findings, finding)
	}
	result.Status = combineStatuses(statuses...)
	return result
}

func lockfileForEcosystem(r *repo.Repo, ecosystem repo.Ecosystem) (Status, Finding) {
	switch ecosystem {
	case repo.EcosystemGo:
		if repo.HasFile(r.Files, "go.sum") {
			return StatusPass, Finding{File: "go.sum", Message: "Go checksum file found"}
		}
		return StatusFail, Finding{
			File:    "go.mod",
			Message: "go.sum is missing",
			Hint:    "run: go mod tidy && git add go.sum",
		}
	case repo.EcosystemNode:
		for _, lockfile := range []string{"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock"} {
			if repo.HasFile(r.Files, lockfile) {
				return StatusPass, Finding{File: lockfile, Message: "Node dependency lockfile found"}
			}
		}
		return StatusFail, Finding{
			File:    "package.json",
			Message: "no npm, pnpm, Yarn, or Bun lockfile found",
			Hint:    "run: npm install && git add package-lock.json",
		}
	case repo.EcosystemPython:
		for _, lockfile := range []string{"uv.lock", "poetry.lock"} {
			if repo.HasFile(r.Files, lockfile) {
				return StatusPass, Finding{File: lockfile, Message: "Python dependency lockfile found"}
			}
		}
		if !repo.HasFile(r.Files, "requirements.txt") {
			return StatusFail, Finding{
				File:    "pyproject.toml",
				Message: "no uv.lock, poetry.lock, or pinned requirements.txt found",
				Hint:    "run: uv lock && git add uv.lock",
			}
		}
		contents, err := r.Read("requirements.txt")
		if err != nil {
			return StatusFail, Finding{
				File:    "requirements.txt",
				Message: fmt.Sprintf("requirements.txt could not be read: %v", err),
				Hint:    "run: uv lock && git add uv.lock",
			}
		}
		line, dependency, pinned := firstUnpinnedRequirement(string(contents))
		if pinned {
			return StatusPass, Finding{File: "requirements.txt", Message: "all requirements are pinned with =="}
		}
		if line == 0 {
			return StatusFail, Finding{
				File:    "requirements.txt",
				Message: "requirements.txt contains no pinned dependencies",
				Hint:    "run: uv lock && git add uv.lock",
			}
		}
		return StatusWarn, Finding{
			File:    "requirements.txt",
			Line:    line,
			Message: fmt.Sprintf("dependency %q is not pinned with ==", dependency),
			Hint:    "pin every dependency or run: uv lock",
		}
	default:
		return StatusSkip, Finding{Message: "unsupported ecosystem"}
	}
}

func firstUnpinnedRequirement(contents string) (lineNumber int, dependency string, allPinned bool) {
	seenDependency := false
	allPinned = true
	for index, rawLine := range strings.Split(contents, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		seenDependency = true
		if !strings.Contains(requirementSpecifier(line), "==") {
			return index + 1, line, false
		}
	}
	if !seenDependency {
		return 0, "", false
	}
	return 0, "", allPinned
}

// requirementSpecifier strips the inline comment and environment marker from a
// requirements.txt line so that "==" inside them is not mistaken for a pin.
func requirementSpecifier(line string) string {
	if index := inlineComment.FindStringIndex(line); index != nil {
		line = line[:index[0]]
	}
	if index := strings.Index(line, ";"); index >= 0 {
		line = line[:index]
	}
	return line
}
