package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func BenchmarkLoadTrackedTree(b *testing.B) {
	root := b.TempDir()
	for index := 0; index < 1000; index++ {
		path := filepath.Join(root, "src", fmt.Sprintf("pkg-%03d", index%50), fmt.Sprintf("file-%04d.txt", index))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("rebuildable benchmark text\n"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/benchmark\ngo 1.27\ntoolchain go1.27.2\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), nil, 0o644); err != nil {
		b.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "benchmark@example.invalid"},
		{"config", "user.name", "Rebuildable Benchmark"},
		{"add", "."},
		{"commit", "-q", "-m", "benchmark"},
	} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			b.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	for index := 0; index < 2; index++ {
		command := exec.Command("git", "commit", "--allow-empty", "-q", "-m", fmt.Sprintf("history-%d", index))
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			b.Fatalf("git history: %v\n%s", err, output)
		}
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		loaded, cleanup, err := Load(b.Context(), root)
		if err != nil {
			b.Fatal(err)
		}
		if len(loaded.Files) == 0 {
			b.Fatal("benchmark loaded no files")
		}
		cleanup()
	}
}
