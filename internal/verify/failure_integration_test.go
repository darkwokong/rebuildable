//go:build integration

package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
	"github.com/jayeshsarvaiya/rebuildable/internal/testutil"
)

func TestVerifyReportsTestFailureAndKeepsLogs(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	writeVerifyFile(t, root, "failure_test.go", `package main

import "testing"

func TestIntentionalFailure(t *testing.T) { t.Fatal("intentional verify failure") }
`)
	testutil.InitGitRepo(t, root, 3)
	workspace, cleanup, err := repo.LoadWorkspace(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	report, err := Run(context.Background(), workspace, Options{Timeout: 5 * time.Minute, Port: 18080, KeepLogs: true, Trusted: true})
	if err != nil {
		t.Fatalf("verify returned tool error: %v", err)
	}
	if report.Passed() || len(report.Stages) < 4 || report.Stages[len(report.Stages)-1].Passed {
		t.Fatalf("failure report = %+v", report)
	}
	if _, err := os.Stat(report.Stages[len(report.Stages)-1].LogPath); err != nil {
		t.Fatalf("test log missing: %v", err)
	}
}

func writeVerifyFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
