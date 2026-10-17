package checks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
	"github.com/jayeshsarvaiya/rebuildable/internal/testutil"
)

func TestNonGitWorkspaceAuditsPresentFiles(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	loaded, cleanup, err := repo.Load(context.Background(), root)
	if err != nil {
		t.Fatalf("load non-Git workspace: %v", err)
	}
	defer cleanup()
	results := RunAll(context.Background(), loaded)
	if Score(results) < 5 {
		t.Fatalf("non-Git score = %d, want present workspace files to be audited: %+v", Score(results), results)
	}
	if results[1].Status != StatusPass || results[2].Status != StatusPass || results[3].Status != StatusPass || results[5].Status != StatusPass {
		t.Fatalf("present workspace checks were not evaluated: %+v", results)
	}
}

func TestReadmeCheckAndFixShareFencedHeadingSemantics(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, root, "README.md", "# Example\n\n```md\n## Setup\n## Run\n## Test\n```\n")
	writeQualityFile(t, root, "go.mod", "module example.com/readme\ngo 1.27\ntoolchain go1.27.2\n")
	writeQualityFile(t, root, "go.sum", "")
	testutil.InitGitRepo(t, root, 3)
	loaded, cleanup, err := repo.Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	result := (readmeCheck{}).Run(context.Background(), loaded)
	if result.Status != StatusFail {
		t.Fatalf("fenced README status = %s, want fail", result.Status)
	}
}

func writeQualityFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
