package repo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
	"github.com/jayeshsarvaiya/rebuildable/internal/testutil"
)

func TestLoadFixture(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	testutil.InitGitRepo(t, root, 3)

	loaded, cleanup, err := repo.Load(t.Context(), root)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	defer cleanup()

	if !loaded.GitAvailable {
		t.Fatal("expected Git repository")
	}
	if loaded.CommitCount != 3 {
		t.Fatalf("commit count = %d, want 3", loaded.CommitCount)
	}
	if len(loaded.Ecosystems) != 1 || loaded.Ecosystems[0] != repo.EcosystemGo {
		t.Fatalf("ecosystems = %v, want [go]", loaded.Ecosystems)
	}
	for _, name := range []string{"go.mod", "go.sum", "main.go"} {
		if !repo.HasFile(loaded.Files, name) {
			t.Fatalf("tracked files %v do not contain %q", loaded.Files, name)
		}
	}
}

func TestLoadScopesTrackedFilesToRequestedSubdirectory(t *testing.T) {
	parent := t.TempDir()
	fixture := testutil.CopyFixture(t, "good-go")
	scope := filepath.Join(parent, "nested")
	if err := os.Rename(fixture, scope); err != nil {
		t.Fatalf("move fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	testutil.InitGitRepo(t, parent, 3)

	loaded, cleanup, err := repo.Load(t.Context(), scope)
	if err != nil {
		t.Fatalf("load scoped fixture: %v", err)
	}
	defer cleanup()

	if loaded.Root != scope {
		t.Fatalf("root = %q, want %q", loaded.Root, scope)
	}
	if repo.HasFile(loaded.Files, "outside.txt") {
		t.Fatalf("scope leaked outside file: %v", loaded.Files)
	}
	if !repo.HasFile(loaded.Files, "go.mod") {
		t.Fatalf("scoped files = %v, want go.mod", loaded.Files)
	}
}

func TestLoadSkipsBinaryAndLargeFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.dat"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, (1<<20)+1), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.InitGitRepo(t, root, 1)

	loaded, cleanup, err := repo.Load(t.Context(), root)
	if err != nil {
		t.Fatalf("load repository: %v", err)
	}
	defer cleanup()

	if !repo.HasFile(loaded.Files, "go.mod") {
		t.Fatalf("scannable files = %v, want go.mod", loaded.Files)
	}
	for _, excluded := range []string{"binary.dat", "large.txt"} {
		if repo.HasFile(loaded.Files, excluded) {
			t.Fatalf("scannable files unexpectedly contain %q: %v", excluded, loaded.Files)
		}
	}
}

func TestLoadNonGitDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, cleanup, err := repo.Load(t.Context(), root)
	if err != nil {
		t.Fatalf("load non-Git directory: %v", err)
	}
	defer cleanup()
	if loaded.GitAvailable {
		t.Fatal("expected Git to be unavailable")
	}
	if len(loaded.Files) != 0 {
		t.Fatalf("files = %v, want no tracked files", loaded.Files)
	}
}

func TestReadRejectsPathEscape(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	testutil.InitGitRepo(t, root, 1)
	loaded, cleanup, err := repo.Load(t.Context(), root)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	defer cleanup()

	if _, err := loaded.Read("../outside"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("Read escape error = %v", err)
	}
}

func TestDetectEcosystems(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []repo.Ecosystem
	}{
		{name: "go", files: []string{"go.mod"}, want: []repo.Ecosystem{repo.EcosystemGo}},
		{name: "node", files: []string{"package.json", "package-lock.json"}, want: []repo.Ecosystem{repo.EcosystemNode}},
		{name: "python", files: []string{"requirements.txt"}, want: []repo.Ecosystem{repo.EcosystemPython}},
		{name: "multiple", files: []string{"go.mod", "package.json", "pyproject.toml"}, want: []repo.Ecosystem{repo.EcosystemGo, repo.EcosystemNode, repo.EcosystemPython}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := repo.DetectEcosystems(test.files)
			if strings.Join(ecosystemNames(got), ",") != strings.Join(ecosystemNames(test.want), ",") {
				t.Fatalf("ecosystems = %v, want %v", got, test.want)
			}
		})
	}
}

func ecosystemNames(ecosystems []repo.Ecosystem) []string {
	names := make([]string, 0, len(ecosystems))
	for _, ecosystem := range ecosystems {
		names = append(names, ecosystem.String())
	}
	return names
}
