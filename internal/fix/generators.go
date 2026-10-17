package fix

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

const (
	defaultGoVersion     = "1.27.2"
	defaultNodeVersion   = "20.11.1"
	defaultPythonVersion = "3.12.1"
)

var exactVersionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
var envExampleKeyPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=`)

func dockerfileArtifacts(workspace *repo.Workspace, ecosystem repo.Ecosystem) ([]artifact, error) {
	artifacts := make([]artifact, 0, 2)
	if !workspace.HasFile("Dockerfile") {
		version, err := runtimeVersion(workspace, ecosystem)
		if err != nil {
			return nil, err
		}
		data := map[string]string{"Version": version, "EntryPoint": pythonEntryPoint(workspace), "BuildTarget": goBuildTarget(workspace), "InstallCommand": nodeInstallCommand(workspace), "Lockfile": nodeLockfile(workspace)}
		var templateName string
		switch ecosystem {
		case repo.EcosystemGo:
			if data["BuildTarget"] == "" {
				return nil, nil
			}
			templateName = "docker-go.tmpl"
		case repo.EcosystemNode:
			if nodeStartScript(workspace) == "" || nodeLockfile(workspace) == "" {
				// A Dockerfile with an invented start command is less useful than a
				// manual-action finding; independent generators still run below.
				templateName = ""
			} else {
				templateName = "docker-node.tmpl"
			}
		case repo.EcosystemPython:
			if data["EntryPoint"] != "" {
				templateName = "docker-python.tmpl"
			}
		default:
			return nil, fmt.Errorf("unsupported ecosystem %q", ecosystem)
		}
		if templateName != "" {
			dockerfile, err := renderTemplate(templateName, data)
			if err != nil {
				return nil, err
			}
			artifacts = append(artifacts, artifact{Path: "Dockerfile", Content: dockerfile, Mode: 0o644})
		}
	}
	if !workspace.HasFile(".dockerignore") {
		ignore, err := renderTemplate("dockerignore.tmpl", nil)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact{Path: ".dockerignore", Content: ignore, Mode: 0o644})
	}
	return artifacts, nil
}

func environmentArtifacts(workspace *repo.Workspace) ([]artifact, error) {
	snapshot := &repo.Repo{Files: workspace.Files, Read: workspace.Read}
	names := checks.DetectedEnvironmentVariables(snapshot)
	var existing []byte
	if workspace.HasFile(".env.example") {
		var err error
		existing, err = workspace.Read(".env.example")
		if err != nil {
			return nil, err
		}
	}
	if len(names) == 0 && len(existing) > 0 {
		return nil, nil
	}
	keys := make(map[string]bool)
	for _, match := range envExampleKeyPattern.FindAllStringSubmatch(string(existing), -1) {
		if len(match) > 1 {
			keys[match[1]] = true
		}
	}
	var output strings.Builder
	output.Write(existing)
	if output.Len() > 0 && !strings.HasSuffix(output.String(), "\n") {
		output.WriteByte('\n')
	}
	if len(names) == 0 && output.Len() == 0 {
		output.WriteString("# No environment variables detected.\n")
	}
	for _, name := range names {
		if keys[name] {
			continue
		}
		output.WriteString(name)
		output.WriteString("=\n")
	}
	return []artifact{{Path: ".env.example", Content: []byte(output.String()), Mode: 0o644}}, nil
}

func pinArtifacts(workspace *repo.Workspace, ecosystem repo.Ecosystem) ([]artifact, error) {
	switch ecosystem {
	case repo.EcosystemGo:
		if !workspace.HasFile("go.mod") {
			return nil, nil
		}
		contents, err := workspace.Read("go.mod")
		if err != nil {
			return nil, err
		}
		if hasExactGoToolchain(string(contents)) {
			return nil, nil
		}
		version := goVersion(string(contents))
		if version == "" {
			version = defaultGoVersion
		}
		return []artifact{{Path: "go.mod", Content: insertGoToolchain(contents, version), Mode: 0o644}}, nil
	case repo.EcosystemNode:
		if hasExactRuntimeFile(workspace, ".nvmrc") || hasExactRuntimeFile(workspace, ".node-version") || hasExactToolVersion(workspace, "nodejs") {
			return nil, nil
		}
		return []artifact{{Path: ".nvmrc", Content: []byte(runtimeVersionOrDefault(workspace, ecosystem, defaultNodeVersion) + "\n"), Mode: 0o644}}, nil
	case repo.EcosystemPython:
		if hasExactRuntimeFile(workspace, ".python-version") || hasExactToolVersion(workspace, "python") || exactPythonRequirement(workspace) != "" {
			return nil, nil
		}
		return []artifact{{Path: ".python-version", Content: []byte(runtimeVersionOrDefault(workspace, ecosystem, defaultPythonVersion) + "\n"), Mode: 0o644}}, nil
	default:
		return nil, fmt.Errorf("unsupported ecosystem %q", ecosystem)
	}
}

func testArtifacts(workspace *repo.Workspace, ecosystem repo.Ecosystem) ([]artifact, error) {
	snapshot := workspaceRepo(workspace)
	if _, ok := checks.DetectTestCommand(snapshot); ok {
		return nil, nil
	}
	if ecosystem == repo.EcosystemNode || ecosystem == repo.EcosystemPython {
		return nil, nil
	}
	if workspace.HasFile("Makefile") {
		contents, err := workspace.Read("Makefile")
		if err != nil {
			return nil, err
		}
		if hasMakeTestTarget(string(contents)) {
			return nil, nil
		}
		addition, err := renderMakefile(ecosystem)
		if err != nil {
			return nil, err
		}
		separator := "\n"
		if len(contents) == 0 || strings.HasSuffix(string(contents), "\n") {
			separator = ""
		}
		return []artifact{{Path: "Makefile", Content: append(append([]byte{}, contents...), append([]byte(separator), addition...)...), Mode: 0o644}}, nil
	}
	content, err := renderMakefile(ecosystem)
	if err != nil {
		return nil, err
	}
	return []artifact{{Path: "Makefile", Content: content, Mode: 0o644}}, nil
}

func readmeArtifacts(workspace *repo.Workspace, ecosystem repo.Ecosystem) ([]artifact, error) {
	content, err := renderTemplate("readme.tmpl", map[string]string{"Ecosystem": string(ecosystem), "TestCommand": readmeTestCommand(workspace, ecosystem)})
	if err != nil {
		return nil, err
	}
	if !workspace.HasFile("README.md") {
		return []artifact{{Path: "README.md", Content: content, Mode: 0o644}}, nil
	}
	existing, err := workspace.Read("README.md")
	if err != nil {
		return nil, err
	}
	missing := missingReadmeSections(string(existing))
	if len(missing) == 0 {
		return nil, nil
	}
	var addition strings.Builder
	for _, section := range missing {
		addition.WriteString("\n## ")
		addition.WriteString(section)
		addition.WriteString("\n\nSee the generated ")
		addition.WriteString(strings.ToLower(section))
		addition.WriteString(" command for this project.\n")
	}
	return []artifact{{Path: "README.md", Content: append(append([]byte{}, existing...), []byte(addition.String())...), Mode: 0o644}}, nil
}

func readmeTestCommand(workspace *repo.Workspace, ecosystem repo.Ecosystem) string {
	switch ecosystem {
	case repo.EcosystemGo:
		return "Run `make test` (which executes `go test ./...`)."
	case repo.EcosystemNode:
		if nodeTestScript(workspace) != "" {
			return "Run `make test` (which executes `npm test`)."
		}
		return "No safe test command was detected; add a real `scripts.test` command before running verification."
	case repo.EcosystemPython:
		if pythonTestsDetected(workspace) {
			return "Run `make test` (which executes `python -m pytest`)."
		}
		return "No safe pytest configuration and test files were detected; add them before running verification."
	default:
		return "Add the project's test command before running verification."
	}
}

func runtimeVersion(workspace *repo.Workspace, ecosystem repo.Ecosystem) (string, error) {
	fallback := map[repo.Ecosystem]string{
		repo.EcosystemGo:     defaultGoVersion,
		repo.EcosystemNode:   defaultNodeVersion,
		repo.EcosystemPython: defaultPythonVersion,
	}[ecosystem]
	version := runtimeVersionOrDefault(workspace, ecosystem, fallback)
	if version == "" {
		return "", fmt.Errorf("could not determine an exact %s runtime version", ecosystem)
	}
	return version, nil
}

func runtimeVersionOrDefault(workspace *repo.Workspace, ecosystem repo.Ecosystem, fallback string) string {
	var files []string
	switch ecosystem {
	case repo.EcosystemNode:
		files = []string{".nvmrc", ".node-version"}
	case repo.EcosystemPython:
		files = []string{".python-version"}
	}
	for _, file := range files {
		if !workspace.HasFile(file) {
			continue
		}
		contents, err := workspace.Read(file)
		if err == nil {
			version := strings.TrimSpace(string(contents))
			if exactVersionPattern.MatchString(version) {
				return strings.TrimPrefix(version, "v")
			}
		}
	}
	if workspace.HasFile(".tool-versions") {
		toolName := map[repo.Ecosystem]string{repo.EcosystemNode: "nodejs", repo.EcosystemPython: "python"}[ecosystem]
		if toolName != "" {
			if contents, err := workspace.Read(".tool-versions"); err == nil {
				for _, line := range strings.Split(string(contents), "\n") {
					fields := strings.Fields(line)
					if len(fields) >= 2 && fields[0] == toolName && exactVersionPattern.MatchString(fields[1]) {
						return strings.TrimPrefix(fields[1], "v")
					}
				}
			}
		}
	}
	if ecosystem == repo.EcosystemGo && workspace.HasFile("go.mod") {
		if contents, err := workspace.Read("go.mod"); err == nil {
			if version := goVersion(string(contents)); version != "" {
				return normalizeRuntimeVersion(version, defaultGoVersion)
			}
		}
	}
	if workspace.HasFile("package.json") && ecosystem == repo.EcosystemNode {
		if contents, err := workspace.Read("package.json"); err == nil {
			var packageJSON struct {
				Engines struct {
					Node string `json:"node"`
				} `json:"engines"`
			}
			if json.Unmarshal(contents, &packageJSON) == nil && exactVersionPattern.MatchString(packageJSON.Engines.Node) {
				return strings.TrimPrefix(packageJSON.Engines.Node, "v")
			}
		}
	}
	return fallback
}

func normalizeRuntimeVersion(value, fallback string) string {
	value = strings.TrimPrefix(value, "v")
	if exactVersionPattern.MatchString(value) {
		return value
	}
	return fallback
}

func goVersion(contents string) string {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	return ""
}

func hasDirective(contents, name string) bool {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name {
			return true
		}
	}
	return false
}

func insertGoToolchain(contents []byte, version string) []byte {
	lines := strings.Split(string(contents), "\n")
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			insertion := append([]string{}, lines[:index+1]...)
			insertion = append(insertion, "toolchain go"+normalizeRuntimeVersion(version, defaultGoVersion))
			insertion = append(insertion, lines[index+1:]...)
			return []byte(strings.Join(insertion, "\n"))
		}
	}
	return append(contents, []byte("\ntoolchain go"+normalizeRuntimeVersion(version, defaultGoVersion)+"\n")...)
}

func hasExactRuntimeFile(workspace *repo.Workspace, file string) bool {
	if !workspace.HasFile(file) {
		return false
	}
	contents, err := workspace.Read(file)
	return err == nil && exactVersionPattern.MatchString(strings.TrimSpace(string(contents)))
}

func hasExactToolVersion(workspace *repo.Workspace, name string) bool {
	if !workspace.HasFile(".tool-versions") {
		return false
	}
	contents, err := workspace.Read(".tool-versions")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name && exactVersionPattern.MatchString(fields[1]) {
			return true
		}
	}
	return false
}

func exactPythonRequirement(workspace *repo.Workspace) string {
	if !workspace.HasFile("pyproject.toml") {
		return ""
	}
	contents, err := workspace.Read("pyproject.toml")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if !strings.Contains(line, "requires-python") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")
		if strings.HasPrefix(value, "==") && exactVersionPattern.MatchString(strings.TrimPrefix(value, "==")) {
			return strings.TrimPrefix(value, "==")
		}
	}
	return ""
}

func hasExactGoToolchain(contents string) bool {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "toolchain" && exactVersionPattern.MatchString(strings.TrimPrefix(fields[1], "go")) && strings.HasPrefix(fields[1], "go") {
			return true
		}
	}
	return false
}

func pythonEntryPoint(workspace *repo.Workspace) string {
	for _, file := range []string{"app.py", "main.py"} {
		if workspace.HasFile(file) {
			return file
		}
	}
	return ""
}

func goBuildTarget(workspace *repo.Workspace) string {
	if workspace.HasFile("main.go") {
		return "."
	}
	targets := make([]string, 0)
	for _, file := range workspace.Files {
		if !strings.HasPrefix(file, "cmd/") || !strings.HasSuffix(file, "/main.go") {
			continue
		}
		parts := strings.Split(file, "/")
		if len(parts) == 3 {
			targets = append(targets, "./cmd/"+parts[1])
		}
	}
	if len(targets) == 1 {
		return targets[0]
	}
	return ""
}

func nodeInstallCommand(workspace *repo.Workspace) string {
	switch nodeLockfile(workspace) {
	case "package-lock.json":
		return "npm ci"
	case "pnpm-lock.yaml":
		return "corepack enable && pnpm install --frozen-lockfile"
	case "yarn.lock":
		return "corepack enable && yarn install --immutable"
	case "bun.lock":
		return "corepack enable && bun install --frozen-lockfile"
	default:
		return ""
	}
}

func nodeLockfile(workspace *repo.Workspace) string {
	for _, file := range []string{"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock"} {
		if workspace.HasFile(file) {
			return file
		}
	}
	return ""
}

func nodeStartScript(workspace *repo.Workspace) string {
	if !workspace.HasFile("package.json") {
		return ""
	}
	contents, err := workspace.Read("package.json")
	if err != nil {
		return ""
	}
	var packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(contents, &packageJSON) != nil {
		return ""
	}
	return strings.TrimSpace(packageJSON.Scripts["start"])
}

func nodeTestScript(workspace *repo.Workspace) string {
	if !workspace.HasFile("package.json") {
		return ""
	}
	contents, err := workspace.Read("package.json")
	if err != nil {
		return ""
	}
	var packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(contents, &packageJSON) != nil {
		return ""
	}
	script := strings.TrimSpace(packageJSON.Scripts["test"])
	if script == "" || strings.Contains(strings.ToLower(script), "no test specified") || strings.Contains(strings.ToLower(script), "error: no test") {
		return ""
	}
	return script
}

func pythonTestsDetected(workspace *repo.Workspace) bool {
	hasConfig := workspace.HasFile("pytest.ini") || workspace.HasFile("tox.ini") || workspace.HasFile("setup.cfg")
	if workspace.HasFile("pyproject.toml") {
		if contents, err := workspace.Read("pyproject.toml"); err == nil && strings.Contains(string(contents), "[tool.pytest") {
			hasConfig = true
		}
	}
	if !hasConfig {
		return false
	}
	for _, file := range workspace.Files {
		base := filepath.Base(file)
		if strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")) {
			return true
		}
	}
	return false
}

func renderMakefile(ecosystem repo.Ecosystem) ([]byte, error) {
	name := map[repo.Ecosystem]string{
		repo.EcosystemGo:     "make-go.tmpl",
		repo.EcosystemNode:   "make-node.tmpl",
		repo.EcosystemPython: "make-python.tmpl",
	}[ecosystem]
	if name == "" {
		return nil, fmt.Errorf("unsupported ecosystem %q", ecosystem)
	}
	return renderTemplate(name, nil)
}

func workspaceRepo(workspace *repo.Workspace) *repo.Repo {
	return &repo.Repo{Root: workspace.Root, Files: workspace.Files, Read: workspace.Read, Ecosystems: workspace.Ecosystems, GitAvailable: workspace.GitAvailable, TrackedFiles: workspace.TrackedFiles}
}

func hasMakeTestTarget(contents string) bool {
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "test:") {
			return true
		}
	}
	return false
}

func missingReadmeSections(contents string) []string {
	sections := checks.ReadmeSections(contents)
	missing := make([]string, 0, len(sections))
	for _, section := range []string{"setup", "run", "test"} {
		if !sections[section] {
			missing = append(missing, section)
		}
	}
	return missing
}
