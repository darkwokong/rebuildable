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

func TestGoodGoFixtureDockerVerify(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	testutil.InitGitRepo(t, root, 3)
	workspace, cleanup, err := repo.LoadWorkspace(context.Background(), root)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	defer cleanup()

	report, err := Run(context.Background(), workspace, Options{Timeout: 5 * time.Minute, Port: 18080, Trusted: true})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("verification failed: %+v", report)
	}
}

func TestGoHTTPFixtureDockerVerifyProbesExposedPort(t *testing.T) {
	root := t.TempDir()
	writeIntegrationFile(t, root, "go.mod", "module example.com/http-fixture\n\ngo 1.27\ntoolchain go1.27.2\n")
	writeIntegrationFile(t, root, "go.sum", "")
	writeIntegrationFile(t, root, "main.go", `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "ok") })
	_ = http.ListenAndServe(":18081", nil)
}
`)
	writeIntegrationFile(t, root, "main_test.go", `package main

import "testing"

func TestHTTPFixture(t *testing.T) {}
`)
	writeIntegrationFile(t, root, "Dockerfile", `FROM golang:1.27.2-bookworm AS rebuildable-test
WORKDIR /app
COPY . .
CMD ["go", "test", "./..."]

FROM golang:1.27.2-bookworm AS runtime
WORKDIR /app
COPY . .
EXPOSE 18081
CMD ["go", "run", "."]
`)
	testutil.InitGitRepo(t, root, 3)
	workspace, cleanup, err := repo.LoadWorkspace(context.Background(), root)
	if err != nil {
		t.Fatalf("load HTTP workspace: %v", err)
	}
	defer cleanup()

	report, err := Run(context.Background(), workspace, Options{Timeout: 5 * time.Minute, Port: 18081, KeepLogs: true, Trusted: true})
	if err != nil {
		t.Fatalf("HTTP verify: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("HTTP verification failed: %+v", report)
	}
}

func writeIntegrationFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
