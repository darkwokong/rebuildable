package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type testCommandCheck struct{}

func (testCommandCheck) ID() string   { return "5" }
func (testCommandCheck) Name() string { return "One test command" }

func (testCommandCheck) Run(_ context.Context, r *repo.Repo) Result {
	result := Result{CheckID: "5", Fixable: true}
	if repo.HasFile(r.Files, "Makefile") {
		if line, command := makeTestTarget(r); command != "" {
			result.Status = StatusPass
			result.Findings = []Finding{{File: "Makefile", Line: line, Message: fmt.Sprintf("test target runs %s", command)}}
			return result
		}
	}

	if repo.HasFile(r.Files, "package.json") {
		if line, script, status := nodeTestScript(r); script != "" {
			result.Status = status
			result.Findings = []Finding{{File: "package.json", Line: line, Message: fmt.Sprintf("scripts.test runs %s", script)}}
			if status == StatusFail {
				result.Findings[0].Hint = "replace npm's placeholder with a real test command"
			}
			return result
		}
	}

	for _, file := range r.Files {
		if strings.HasSuffix(file, "_test.go") {
			result.Status = StatusPass
			result.Findings = []Finding{{File: file, Message: "Go test file found; run go test ./..."}}
			return result
		}
	}

	if hasPytestConfig(r) && hasPythonTestFile(r) {
		result.Status = StatusPass
		result.Findings = []Finding{{File: "pytest configuration", Message: "pytest configuration and test files found; run pytest"}}
		return result
	}

	result.Status = StatusFail
	result.Findings = []Finding{{
		File:    "Makefile",
		Message: "no recognized test command found",
		Hint:    "add a Makefile test target or wire the detected ecosystem's test runner",
	}}
	return result
}

func makeTestTarget(r *repo.Repo) (int, string) {
	contents, err := r.Read("Makefile")
	if err != nil {
		return 0, ""
	}
	lines := strings.Split(string(contents), "\n")
	for index, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		if !strings.HasPrefix(line, "test:") && !strings.HasPrefix(line, "test ") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		inline := strings.TrimSpace(strings.SplitN(parts[1], "#", 2)[0])
		if inline != "" && inline != ";" {
			return index + 1, inline
		}
		for _, recipe := range lines[index+1:] {
			if strings.TrimSpace(recipe) == "" {
				continue
			}
			if len(recipe) == 0 || (recipe[0] != '\t' && recipe[0] != ' ') {
				break
			}
			command := strings.TrimSpace(strings.SplitN(recipe, "#", 2)[0])
			if command != "" {
				return index + 1, command
			}
		}
	}
	return 0, ""
}

func nodeTestScript(r *repo.Repo) (int, string, Status) {
	contents, err := r.Read("package.json")
	if err != nil {
		return 0, "", StatusFail
	}
	var packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(contents, &packageJSON); err != nil {
		return 1, "package.json is malformed", StatusFail
	}
	script := strings.TrimSpace(packageJSON.Scripts["test"])
	if script == "" {
		return 0, "", StatusFail
	}
	if strings.Contains(strings.ToLower(script), "no test specified") || strings.Contains(strings.ToLower(script), "error: no test") {
		return 1, script, StatusFail
	}
	return 1, script, StatusPass
}

func hasPytestConfig(r *repo.Repo) bool {
	for _, file := range []string{"pytest.ini", "tox.ini", "setup.cfg"} {
		if repo.HasFile(r.Files, file) {
			return true
		}
	}
	if !repo.HasFile(r.Files, "pyproject.toml") {
		return false
	}
	contents, err := r.Read("pyproject.toml")
	return err == nil && strings.Contains(string(contents), "[tool.pytest")
}

func hasPythonTestFile(r *repo.Repo) bool {
	for _, file := range r.Files {
		base := filepath.Base(file)
		if strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")) {
			return true
		}
	}
	return false
}
