package checks

import (
	"context"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type Status int

const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
	StatusSkip
)

func (s Status) String() string {
	switch s {
	case StatusPass:
		return "pass"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	case StatusSkip:
		return "skip"
	default:
		return "unknown"
	}
}

type Finding struct {
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type Result struct {
	CheckID  string    `json:"check_id"`
	Status   Status    `json:"status"`
	Findings []Finding `json:"findings,omitempty"`
	Fixable  bool      `json:"fixable"`
}

type Check interface {
	ID() string
	Name() string
	Run(context.Context, *repo.Repo) Result
}

func Registry() []Check {
	return []Check{
		gitRepoCheck{},
		lockfileCheck{},
		runtimeCheck{},
		notImplementedCheck{id: "4", name: "Dockerfile builds and runs"},
		notImplementedCheck{id: "5", name: "One test command"},
		notImplementedCheck{id: "6", name: "README with setup, run, test"},
		notImplementedCheck{id: "7", name: ".env.example / secrets"},
		notImplementedCheck{id: "8", name: "No absolute paths"},
	}
}

func RunAll(ctx context.Context, r *repo.Repo) []Result {
	results := make([]Result, 0, len(Registry()))
	for _, check := range Registry() {
		results = append(results, check.Run(ctx, r))
	}
	return results
}

func NameForID(id string) string {
	for _, check := range Registry() {
		if check.ID() == id {
			return check.Name()
		}
	}
	return id
}

func Score(results []Result) int {
	score := 0
	for _, result := range results {
		if result.Status == StatusPass {
			score++
		}
	}
	return score
}

func combineStatuses(statuses ...Status) Status {
	seenWarn := false
	seenSkip := false
	for _, status := range statuses {
		switch status {
		case StatusFail:
			return StatusFail
		case StatusWarn:
			seenWarn = true
		case StatusSkip:
			seenSkip = true
		}
	}
	if seenWarn {
		return StatusWarn
	}
	if seenSkip {
		return StatusSkip
	}
	return StatusPass
}
