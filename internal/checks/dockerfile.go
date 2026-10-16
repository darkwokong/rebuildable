package checks

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type dockerfileCheck struct{}

func (dockerfileCheck) ID() string   { return "4" }
func (dockerfileCheck) Name() string { return "Dockerfile builds and runs" }

func (dockerfileCheck) Run(_ context.Context, r *repo.Repo) Result {
	result := Result{CheckID: "4", Fixable: true}
	if !repo.HasFile(r.Files, "Dockerfile") {
		result.Status = StatusFail
		result.Findings = []Finding{{
			File:    "Dockerfile",
			Message: "Dockerfile is missing",
			Hint:    "add a Dockerfile with FROM and CMD or ENTRYPOINT",
		}}
		return result
	}

	contents, err := r.Read("Dockerfile")
	if err != nil {
		result.Status = StatusFail
		result.Findings = []Finding{{
			File:    "Dockerfile",
			Message: fmt.Sprintf("Dockerfile could not be read: %v", err),
			Hint:    "add a readable Dockerfile",
		}}
		return result
	}

	hasFrom, fromLine := directiveLine(string(contents), "FROM")
	hasRunCommand, commandLine := directiveLine(string(contents), "CMD")
	if !hasRunCommand {
		hasRunCommand, commandLine = directiveLine(string(contents), "ENTRYPOINT")
	}

	if !hasFrom {
		result.Status = StatusFail
		result.Findings = append(result.Findings, Finding{
			File:    "Dockerfile",
			Message: "Dockerfile has no FROM instruction",
			Hint:    "add a base image with FROM",
		})
	}
	if !hasRunCommand {
		result.Status = StatusFail
		result.Findings = append(result.Findings, Finding{
			File:    "Dockerfile",
			Message: "Dockerfile has no CMD or ENTRYPOINT instruction",
			Hint:    "add CMD or ENTRYPOINT for the container's default process",
		})
	}
	if hasFrom && hasRunCommand {
		result.Findings = append(result.Findings, Finding{
			File:    "Dockerfile",
			Line:    fromLine,
			Message: fmt.Sprintf("static FROM and run instructions found (run instruction at line %d)", commandLine),
		})
	}
	if !repo.HasFile(r.Files, ".dockerignore") && result.Status != StatusFail {
		result.Status = StatusWarn
		result.Findings = append(result.Findings, Finding{
			File:    ".dockerignore",
			Message: ".dockerignore is missing",
			Hint:    "add .dockerignore so dependencies, secrets, and build output stay out of the image",
		})
	}
	if result.Status == 0 && len(result.Findings) > 0 {
		result.Status = StatusPass
	}
	if !hasFrom || !hasRunCommand {
		result.Status = StatusFail
	}
	return result
}

func directiveLine(contents, directive string) (bool, int) {
	scanner := bufio.NewScanner(strings.NewReader(contents))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.EqualFold(fields[0], directive) {
			return true, lineNumber
		}
	}
	return false, 0
}
