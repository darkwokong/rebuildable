package verify

import (
	"bytes"
	"strings"
	"testing"

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
			command, err := testCommand(&repo.Workspace{Ecosystems: test.ecosystems})
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
