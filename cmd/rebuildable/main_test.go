package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jayeshsarvaiya/rebuildable/internal/report"
	"github.com/jayeshsarvaiya/rebuildable/internal/testutil"
)

func TestAuditCommand(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	testutil.InitGitRepo(t, root, 3)

	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"audit", root})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute audit: %v", err)
	}
	if !strings.Contains(output.String(), "Score 8/8") {
		t.Fatalf("audit output does not contain score:\n%s", output.String())
	}
}

func TestTerminalColorOnlyTargetsCharacterDevices(t *testing.T) {
	if terminalColor(&bytes.Buffer{}) {
		t.Fatal("buffer should not be treated as a color terminal")
	}
}

func TestAuditJSONAndMarkdownFlags(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	testutil.InitGitRepo(t, root, 3)
	markdownPath := filepath.Join(t.TempDir(), "audit.md")

	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"audit", root, "--json", "--md", markdownPath, "--min-score", "8"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute JSON audit: %v", err)
	}
	var document report.Document
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("parse command JSON: %v\n%s", err, output.String())
	}
	if document.Score != 8 || document.MinimumScore == nil || *document.MinimumScore != 8 {
		t.Fatalf("JSON document = %+v", document)
	}
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read Markdown report: %v", err)
	}
	if !strings.Contains(string(markdown), "# Rebuildable report") {
		t.Fatalf("Markdown report = %s", markdown)
	}
}

func TestMinimumScoreReturnsExitCodeOneAfterRendering(t *testing.T) {
	root := testutil.CopyFixture(t, "bad-node")
	testutil.InitGitRepo(t, root, 3)

	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"audit", root, "--min-score", "8"})
	err := command.Execute()
	if err == nil {
		t.Fatal("expected minimum score failure")
	}
	var coded *exitError
	if !errors.As(err, &coded) || coded.code != 1 {
		t.Fatalf("error = %v, want exit code 1", err)
	}
	if !strings.Contains(output.String(), "Score 3/8") {
		t.Fatalf("report was not rendered before threshold error:\n%s", output.String())
	}
}

func TestInvalidMinimumScoreReturnsToolErrorCode(t *testing.T) {
	command := newRootCommand()
	command.SetArgs([]string{"audit", ".", "--min-score", "9"})
	err := command.Execute()
	if err == nil || exitCode(err) != 2 {
		t.Fatalf("error = %v, want tool error code 2", err)
	}
}
