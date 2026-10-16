package checks

import (
	"context"
	"fmt"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type gitRepoCheck struct{}

func (gitRepoCheck) ID() string   { return "1" }
func (gitRepoCheck) Name() string { return "Git repo with commits" }

func (gitRepoCheck) Run(_ context.Context, r *repo.Repo) Result {
	result := Result{CheckID: "1"}
	if !r.GitAvailable {
		result.Status = StatusFail
		result.Findings = []Finding{{
			File:    ".git",
			Message: "directory is not inside a Git repository",
			Hint:    "run: git init && git add . && git commit -m \"Initial commit\"",
		}}
		return result
	}

	if r.CommitCount == 0 {
		result.Status = StatusFail
		result.Findings = []Finding{{
			File:    ".git",
			Message: "Git repository has no commits",
			Hint:    "run: git add . && git commit -m \"Initial commit\"",
		}}
		return result
	}

	result.Findings = []Finding{{
		File:    ".git",
		Message: fmt.Sprintf("%d commit(s) found", r.CommitCount),
	}}
	if r.CommitCount < 3 {
		result.Status = StatusWarn
		result.Findings[0].Hint = "create at least three meaningful commits before sharing the repository"
		return result
	}
	result.Status = StatusPass
	return result
}
