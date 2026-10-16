package checks

import (
	"context"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type notImplementedCheck struct {
	id   string
	name string
}

func (c notImplementedCheck) ID() string   { return c.id }
func (c notImplementedCheck) Name() string { return c.name }

func (c notImplementedCheck) Run(_ context.Context, _ *repo.Repo) Result {
	return Result{
		CheckID: c.id,
		Status:  StatusSkip,
		Findings: []Finding{{
			Message: "not implemented in Session 1",
		}},
	}
}
