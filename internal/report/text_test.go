package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

func TestRenderText(t *testing.T) {
	repository := &repo.Repo{
		Root:       "/tmp/rebuildable-fixture",
		Ecosystems: []repo.Ecosystem{repo.EcosystemGo},
	}
	results := []checks.Result{
		{CheckID: "1", Status: checks.StatusPass, Findings: []checks.Finding{{File: ".git", Message: "3 commit(s) found"}}},
		{CheckID: "2", Status: checks.StatusFail, Findings: []checks.Finding{{File: "go.mod", Message: "go.sum is missing", Hint: "run: go mod tidy"}}},
		{CheckID: "3", Status: checks.StatusSkip, Findings: []checks.Finding{{Message: "not implemented in Session 1"}}},
	}

	var output bytes.Buffer
	if err := RenderText(&output, repository, results, false); err != nil {
		t.Fatalf("render report: %v", err)
	}
	text := output.String()
	for _, expected := range []string{
		"Rebuildable · /tmp/rebuildable-fixture · go",
		"Git repo with commits",
		"go.mod — go.sum is missing",
		"→ run: go mod tidy",
		"Score 1/8",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("report does not contain %q:\n%s", expected, text)
		}
	}
}
