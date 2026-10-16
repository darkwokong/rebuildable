package report

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

func TestReportGoldens(t *testing.T) {
	repository := &repo.Repo{DisplayName: "fixture", Ecosystems: []repo.Ecosystem{repo.EcosystemGo}}
	results := []checks.Result{
		{CheckID: "1", Status: checks.StatusPass, Findings: []checks.Finding{{File: ".git", Message: "3 commit(s) found"}}},
		{CheckID: "2", Status: checks.StatusFail, Findings: []checks.Finding{{File: "go.mod", Message: "go.sum is missing", Hint: "run: go mod tidy"}}},
		{CheckID: "3", Status: checks.StatusSkip, Findings: []checks.Finding{{Message: "not implemented in Session 1"}}},
	}

	var text bytes.Buffer
	if err := RenderText(&text, repository, results, false); err != nil {
		t.Fatalf("render text: %v", err)
	}
	assertGolden(t, "report-text.txt", text.String())

	var jsonOutput bytes.Buffer
	if err := RenderJSON(&jsonOutput, repository, results, nil); err != nil {
		t.Fatalf("render JSON: %v", err)
	}
	assertGolden(t, "report.json", jsonOutput.String())

	var markdown bytes.Buffer
	if err := RenderMarkdown(&markdown, repository, results, nil); err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
	assertGolden(t, "report.md", markdown.String())
}

func assertGolden(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if actual != string(want) {
		t.Fatalf("%s mismatch:\n--- got ---\n%s--- want ---\n%s", name, actual, want)
	}
}
