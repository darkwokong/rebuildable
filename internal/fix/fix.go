package fix

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

//go:embed templates/*
var templateFiles embed.FS

type Action string

const (
	ActionCreate  Action = "create"
	ActionReplace Action = "replace"
	ActionNoop    Action = "no-op"
)

type Options struct {
	Only      []string
	Force     bool
	Ecosystem string
}

type Change struct {
	Generator  string
	Path       string
	Content    []byte
	Mode       os.FileMode
	Exists     bool
	Before     []byte
	BeforeHash string
	Action     Action
	Reason     string
}

type Plan struct {
	Changes []Change
	Errors  []error
}

type ApplyResult struct {
	Applied   []Change
	Skipped   []Change
	Conflicts []Change
}

var generatorOrder = []string{"dockerfile", "env", "pins", "readme", "test"}

func PlanChanges(ctx context.Context, workspace *repo.Workspace, options Options) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	only, err := selectedGenerators(options.Only)
	if err != nil {
		return Plan{}, err
	}
	ecosystem, err := chooseEcosystem(workspace, options, only)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{Changes: make([]Change, 0)}
	for _, generator := range generatorOrder {
		if !only[generator] {
			continue
		}
		var artifacts []artifact
		switch generator {
		case "dockerfile":
			artifacts, err = dockerfileArtifacts(workspace, ecosystem)
		case "env":
			artifacts, err = environmentArtifacts(workspace)
		case "pins":
			artifacts, err = pinArtifacts(workspace, ecosystem)
		case "readme":
			artifacts, err = readmeArtifacts(workspace, ecosystem)
		case "test":
			artifacts, err = testArtifacts(workspace, ecosystem)
		}
		if err != nil {
			plan.Errors = append(plan.Errors, fmt.Errorf("%s generator: %w", generator, err))
			continue
		}
		for _, item := range artifacts {
			change, changeErr := makeChange(workspace, generator, item)
			if changeErr != nil {
				plan.Errors = append(plan.Errors, changeErr)
				continue
			}
			plan.Changes = append(plan.Changes, change)
		}
	}
	if len(plan.Errors) > 0 {
		return plan, joinErrors(plan.Errors)
	}
	sort.Slice(plan.Changes, func(i, j int) bool {
		if plan.Changes[i].Path == plan.Changes[j].Path {
			return plan.Changes[i].Generator < plan.Changes[j].Generator
		}
		return plan.Changes[i].Path < plan.Changes[j].Path
	})
	if err := rejectDuplicatePaths(plan.Changes); err != nil {
		return plan, err
	}
	return plan, nil
}

type artifact struct {
	Path    string
	Content []byte
	Mode    os.FileMode
}

func selectedGenerators(values []string) (map[string]bool, error) {
	selected := make(map[string]bool, len(generatorOrder))
	if len(values) == 0 {
		for _, generator := range generatorOrder {
			selected[generator] = true
		}
		return selected, nil
	}
	aliases := map[string]string{
		"dockerignore": "dockerfile",
		"env-example":  "env",
		"environment":  "env",
		"runtime":      "pins",
		"runtime-pin":  "pins",
	}
	for _, value := range values {
		for _, raw := range strings.Split(value, ",") {
			name := strings.ToLower(strings.TrimSpace(raw))
			if name == "" {
				continue
			}
			if alias, ok := aliases[name]; ok {
				name = alias
			}
			if !contains(generatorOrder, name) {
				return nil, fmt.Errorf("unknown generator %q; choose from %s", name, strings.Join(generatorOrder, ", "))
			}
			selected[name] = true
		}
	}
	return selected, nil
}

func chooseEcosystem(workspace *repo.Workspace, options Options, selected map[string]bool) (repo.Ecosystem, error) {
	if options.Ecosystem != "" {
		for _, ecosystem := range workspace.Ecosystems {
			if string(ecosystem) == strings.ToLower(options.Ecosystem) {
				return ecosystem, nil
			}
		}
		return "", fmt.Errorf("ecosystem %q was not detected; found %s", options.Ecosystem, ecosystemList(workspace.Ecosystems))
	}
	needsEcosystem := selected["dockerfile"] || selected["pins"] || selected["test"] || selected["readme"]
	if !needsEcosystem {
		return "", nil
	}
	if len(workspace.Ecosystems) == 0 {
		return "", fmt.Errorf("no supported ecosystem detected; specify a project with go.mod, package.json, or Python metadata")
	}
	if len(workspace.Ecosystems) > 1 {
		return "", fmt.Errorf("multiple ecosystems detected (%s); rerun with --ecosystem", ecosystemList(workspace.Ecosystems))
	}
	return workspace.Ecosystems[0], nil
}

func ecosystemList(ecosystems []repo.Ecosystem) string {
	values := make([]string, 0, len(ecosystems))
	for _, ecosystem := range ecosystems {
		values = append(values, string(ecosystem))
	}
	return strings.Join(values, ", ")
}

func makeChange(workspace *repo.Workspace, generator string, item artifact) (Change, error) {
	path, err := safeWorkspacePath(workspace.Root, item.Path)
	if err != nil {
		return Change{}, fmt.Errorf("%s: %w", item.Path, err)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Change{Generator: generator, Path: item.Path, Content: item.Content, Mode: item.Mode, Action: ActionCreate, Reason: "file is missing"}, nil
	}
	if err != nil {
		return Change{}, fmt.Errorf("inspect %s: %w", item.Path, err)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Change{}, fmt.Errorf("refusing to modify %s: target is not a regular file", item.Path)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return Change{}, fmt.Errorf("read %s: %w", item.Path, err)
	}
	change := Change{
		Generator:  generator,
		Path:       item.Path,
		Content:    item.Content,
		Exists:     true,
		Before:     before,
		BeforeHash: hashBytes(before),
		Mode:       info.Mode().Perm(),
		Action:     ActionReplace,
	}
	if bytes.Equal(before, item.Content) {
		change.Action = ActionNoop
		change.Reason = "file already matches generated content"
	} else {
		change.Reason = "file differs; --force is required to replace it"
	}
	return change, nil
}

func safeWorkspacePath(root, relative string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	full := filepath.Join(root, clean)
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolvedFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	relativePath, err := filepath.Rel(resolvedRoot, resolvedFull)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	current := resolvedRoot
	for _, component := range strings.Split(relativePath, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path contains a symlink component")
		}
	}
	return resolvedFull, nil
}

func rejectDuplicatePaths(changes []Change) error {
	for index := 1; index < len(changes); index++ {
		if changes[index-1].Path == changes[index].Path {
			return fmt.Errorf("multiple generators target %s", changes[index].Path)
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hashBytes(contents []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(contents))
}

func joinErrors(errorsList []error) error {
	if len(errorsList) == 0 {
		return nil
	}
	messages := make([]string, 0, len(errorsList))
	for _, err := range errorsList {
		messages = append(messages, err.Error())
	}
	return errors.New(strings.Join(messages, "; "))
}

func renderTemplate(name string, data any) ([]byte, error) {
	contents, err := templateFiles.ReadFile("templates/" + name)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(name).Parse(string(contents))
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeDiffLine(w io.Writer, prefix byte, line string) error {
	_, err := fmt.Fprintf(w, "%c%s\n", prefix, line)
	return err
}

func RenderUnifiedDiff(w io.Writer, plan Plan) error {
	changed := false
	for _, change := range plan.Changes {
		switch change.Action {
		case ActionNoop:
			if _, err := fmt.Fprintf(w, "No change: %s (%s)\n", change.Path, change.Reason); err != nil {
				return err
			}
			continue
		case ActionCreate, ActionReplace:
			changed = true
		default:
			continue
		}
		if change.Action == ActionCreate {
			if _, err := fmt.Fprintf(w, "--- /dev/null\n+++ b/%s\n@@\n", change.Path); err != nil {
				return err
			}
			for _, line := range splitDiffLines(change.Content) {
				if err := writeDiffLine(w, '+', line); err != nil {
					return err
				}
			}
		} else {
			if _, err := fmt.Fprintf(w, "--- a/%s\n+++ b/%s\n@@\n", change.Path, change.Path); err != nil {
				return err
			}
			for _, line := range splitDiffLines(change.Before) {
				if err := writeDiffLine(w, '-', line); err != nil {
					return err
				}
			}
			for _, line := range splitDiffLines(change.Content) {
				if err := writeDiffLine(w, '+', line); err != nil {
					return err
				}
			}
		}
	}
	if !changed {
		_, err := io.WriteString(w, "No generated changes.\n")
		return err
	}
	return nil
}

func splitDiffLines(contents []byte) []string {
	text := strings.TrimSuffix(string(contents), "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

func ApplyChanges(ctx context.Context, workspace *repo.Workspace, plan Plan, force bool) (ApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{}
	for _, change := range plan.Changes {
		if change.Action != ActionReplace || force {
			continue
		}
		if change.Action == ActionNoop {
			continue
		}
		result.Conflicts = append(result.Conflicts, change)
	}
	if len(result.Conflicts) > 0 {
		for _, change := range plan.Changes {
			if change.Action != ActionNoop {
				result.Skipped = append(result.Skipped, change)
			}
		}
		return result, nil
	}

	for _, change := range plan.Changes {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if change.Action == ActionNoop {
			result.Skipped = append(result.Skipped, change)
			continue
		}
		if err := applyChange(workspace, change, force); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, change)
	}
	return result, nil
}

func applyChange(workspace *repo.Workspace, change Change, force bool) error {
	path, err := safeWorkspacePath(workspace.Root, change.Path)
	if err != nil {
		return err
	}
	info, statErr := os.Lstat(path)
	if change.Action == ActionCreate {
		if statErr == nil {
			return fmt.Errorf("%s appeared after planning; rerun fix", change.Path)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspect %s before create: %w", change.Path, statErr)
		}
	} else {
		if statErr != nil {
			return fmt.Errorf("%s disappeared after planning; rerun fix", change.Path)
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to modify %s: target is no longer a regular file", change.Path)
		}
		before, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s before replace: %w", change.Path, readErr)
		}
		if hashBytes(before) != change.BeforeHash {
			return fmt.Errorf("%s changed after planning; rerun fix", change.Path)
		}
		if !force {
			return fmt.Errorf("refusing to replace %s without --force", change.Path)
		}
	}
	mode := change.Mode
	if mode == 0 {
		mode = 0o644
	}
	return atomicWrite(path, change.Content, mode, change.Action == ActionCreate)
}

func atomicWrite(path string, contents []byte, mode os.FileMode, mustCreate bool) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".rebuildable-write-")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set file mode: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if mustCreate {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("target %s appeared during write", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
