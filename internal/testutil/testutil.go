package testutil

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// CopyFixture creates a writable copy of a content-only fixture. Git metadata
// is intentionally created by the test so no nested .git directory is ever
// committed to the project.
func CopyFixture(t *testing.T, name string) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate testutil package")
	}
	fixture := filepath.Join(filepath.Dir(currentFile), "..", "..", "testdata", "fixtures", name)
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("stat fixture %q: %v", name, err)
	}

	destination := filepath.Join(t.TempDir(), name)
	if err := copyTree(destination, fixture); err != nil {
		t.Fatalf("copy fixture %q: %v", name, err)
	}
	return destination
}

func InitGitRepo(t *testing.T, root string, commits int) {
	t.Helper()
	if commits < 1 {
		t.Fatalf("commits must be at least 1, got %d", commits)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "rebuildable-tests@example.invalid")
	runGit(t, root, "config", "user.name", "Rebuildable Tests")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "fixture commit 1")
	for index := 2; index <= commits; index++ {
		runGit(t, root, "commit", "--allow-empty", "-q", "-m", fmt.Sprintf("fixture commit %d", index))
	}
}

func CreateBareClone(t *testing.T, source string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "fixture.git")
	command := exec.Command("git", "clone", "--bare", source, bare)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %v\n%s", err, output)
	}
	return bare
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func copyTree(destination, source string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
}
