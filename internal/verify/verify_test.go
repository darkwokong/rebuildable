package verify

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

func TestTestCommandForEcosystems(t *testing.T) {
	tests := []struct {
		name       string
		ecosystems []repo.Ecosystem
		want       []string
		wantErr    bool
	}{
		{name: "Go", ecosystems: []repo.Ecosystem{repo.EcosystemGo}, want: []string{"go", "test", "./..."}},
		{name: "Node", ecosystems: []repo.Ecosystem{repo.EcosystemNode}, want: []string{"npm", "test"}},
		{name: "Python", ecosystems: []repo.Ecosystem{repo.EcosystemPython}, want: []string{"python", "-m", "pytest"}},
		{name: "ambiguous", ecosystems: []repo.Ecosystem{repo.EcosystemGo, repo.EcosystemNode}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := &repo.Workspace{Ecosystems: test.ecosystems}
			switch test.name {
			case "Go":
				workspace.Files = []string{"main_test.go"}
			case "Node":
				workspace.Files = []string{"package.json"}
				workspace.Read = func(string) ([]byte, error) { return []byte(`{"scripts":{"test":"node test.js"}}`), nil }
			case "Python":
				workspace.Files = []string{"pyproject.toml", "test_smoke.py"}
				workspace.Read = func(path string) ([]byte, error) {
					if path == "pyproject.toml" {
						return []byte("[tool.pytest]\n"), nil
					}
					return nil, os.ErrNotExist
				}
			}
			command, err := testCommand(workspace)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected ambiguous ecosystem error")
				}
				return
			}
			if err != nil || strings.Join(command, " ") != strings.Join(test.want, " ") {
				t.Fatalf("command = %v, err = %v; want %v", command, err, test.want)
			}
		})
	}
}

func TestRenderVerifyReport(t *testing.T) {
	var output bytes.Buffer
	report := Report{ImageTag: "rebuildable-verify-test", LogDir: "logs", Stages: []StageResult{{Name: "build", Passed: true}, {Name: "test", Passed: false, Error: "test failed"}}}
	if err := RenderText(&output, report); err != nil {
		t.Fatalf("render report: %v", err)
	}
	if !strings.Contains(output.String(), "test failed") || !strings.Contains(output.String(), "rebuildable-verify-test") {
		t.Fatalf("report = %s", output.String())
	}
}

func TestPrepareLogDir(t *testing.T) {
	root := t.TempDir()
	path, err := prepareLogDir(root, "logs/run")
	if err != nil {
		t.Fatalf("prepare log directory: %v", err)
	}
	if !strings.HasPrefix(path, root) {
		t.Fatalf("log path %q escaped root %q", path, root)
	}
	if _, err := prepareLogDir(root, "/tmp/outside"); err == nil {
		t.Fatal("absolute log directory unexpectedly accepted")
	}
}

func TestRunRequiresExplicitTrust(t *testing.T) {
	_, err := Run(context.Background(), &repo.Workspace{Root: t.TempDir()}, Options{Timeout: time.Minute, Port: 8080})
	if !errors.Is(err, ErrTrustRequired) {
		t.Fatalf("error = %v, want ErrTrustRequired", err)
	}
}

func TestWaitForHTTPHonorsContextDeadlineBeyondThirtySeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("waits more than 30 seconds")
	}
	started := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if time.Since(started) < 31*time.Second {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := waitForHTTP(ctx, port); err != nil {
		t.Fatalf("waitForHTTP() error = %v, want nil once the server becomes healthy within the context deadline", err)
	}
}

func TestWaitForHTTPReturnsContextErrorOnDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	_, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := waitForHTTP(ctx, port); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForHTTP() error = %v, want context.DeadlineExceeded", err)
	}
}
