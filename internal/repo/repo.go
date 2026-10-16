package repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const maxScannedFileSize int64 = 1 << 20

// Repo is the read-only snapshot used by the audit checks.
type Repo struct {
	Root         string
	GitRoot      string
	Files        []string
	Ecosystems   []Ecosystem
	Read         func(path string) ([]byte, error)
	GitAvailable bool
	CommitCount  int
}

// Load reads a local repository or repository subdirectory. Git metadata is
// optional so the Git check can report a useful failure instead of making the
// entire audit a tool error.
func Load(ctx context.Context, input string) (*Repo, func(), error) {
	if strings.Contains(input, "://") || strings.HasPrefix(input, "git@") {
		return nil, func() {}, fmt.Errorf("git URLs are not supported until the URL loader is implemented")
	}

	root, err := filepath.Abs(input)
	if err != nil {
		return nil, func() {}, fmt.Errorf("resolve repository path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, func() {}, fmt.Errorf("stat repository path: %w", err)
	}
	if !info.IsDir() {
		return nil, func() {}, fmt.Errorf("repository path is not a directory: %s", input)
	}

	gitRoot, gitAvailable, err := findGitRoot(ctx, root)
	if err != nil {
		return nil, func() {}, err
	}

	files := make([]string, 0)
	commitCount := 0
	if gitAvailable {
		files, err = listTrackedFiles(ctx, gitRoot, root)
		if err != nil {
			return nil, func() {}, err
		}
		commitCount, err = gitCommitCount(ctx, gitRoot)
		if err != nil {
			return nil, func() {}, err
		}
	}

	loaded := &Repo{
		Root:         root,
		GitRoot:      gitRoot,
		Files:        files,
		Read:         readFile(root),
		GitAvailable: gitAvailable,
		CommitCount:  commitCount,
	}
	loaded.Ecosystems = DetectEcosystems(loaded.Files)

	return loaded, func() {}, nil
}

func findGitRoot(ctx context.Context, root string) (string, bool, error) {
	output, err := runGit(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", false, err
		}
		return root, false, nil
	}

	gitRoot := strings.TrimSpace(string(output))
	if gitRoot == "" {
		return root, false, nil
	}
	gitRoot, err = filepath.Abs(gitRoot)
	if err != nil {
		return "", false, fmt.Errorf("resolve Git root: %w", err)
	}
	return gitRoot, true, nil
}

func listTrackedFiles(ctx context.Context, gitRoot, scopeRoot string) ([]string, error) {
	args := []string{"ls-files", "-z", "--"}
	relativeScope, err := filepath.Rel(gitRoot, scopeRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository scope: %w", err)
	}
	if relativeScope != "." {
		args = append(args, filepath.ToSlash(relativeScope))
	}

	output, err := runGit(ctx, gitRoot, args...)
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}

	prefix := ""
	if relativeScope != "." {
		prefix = filepath.ToSlash(relativeScope) + "/"
	}
	files := make([]string, 0)
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		name := filepath.ToSlash(string(raw))
		if prefix != "" {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = strings.TrimPrefix(name, prefix)
		}
		if keepScannableFile(filepath.Join(scopeRoot, filepath.FromSlash(name))) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func keepScannableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxScannedFileSize {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	var sample [8192]byte
	count, err := file.Read(sample[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return !bytes.Contains(sample[:count], []byte{0})
}

func gitCommitCount(ctx context.Context, gitRoot string) (int, error) {
	output, err := runGit(ctx, gitRoot, "rev-list", "--count", "HEAD")
	if err != nil {
		if strings.Contains(string(output), "does not have any commits yet") {
			return 0, nil
		}
		// An empty repository returns a non-zero status with the diagnostic on
		// stderr, which runGit includes in the returned error.
		if strings.Contains(err.Error(), "does not have any commits yet") || strings.Contains(err.Error(), "unknown revision") {
			return 0, nil
		}
		return 0, nil
	}

	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count); err != nil {
		return 0, fmt.Errorf("parse Git commit count: %w", err)
	}
	return count, nil
}

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return output, ctx.Err()
		}
		return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func readFile(root string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		clean := filepath.Clean(filepath.FromSlash(path))
		full := filepath.Join(root, clean)
		relative, err := filepath.Rel(root, full)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("path escapes repository root: %s", path)
		}
		return os.ReadFile(full)
	}
}
