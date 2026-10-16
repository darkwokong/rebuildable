package verify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

var ErrDockerUnavailable = errors.New("docker is unavailable")

type Options struct {
	Timeout  time.Duration
	Port     int
	KeepLogs bool
	LogDir   string
}

type StageResult struct {
	Name     string        `json:"name"`
	Passed   bool          `json:"passed"`
	TimedOut bool          `json:"timed_out"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
	LogPath  string        `json:"log_path"`
	Error    string        `json:"error,omitempty"`
}

type Report struct {
	ImageTag     string        `json:"image_tag"`
	TestImageTag string        `json:"test_image_tag,omitempty"`
	LogDir       string        `json:"log_dir"`
	Stages       []StageResult `json:"stages"`
}

func (r Report) Passed() bool {
	if len(r.Stages) == 0 {
		return false
	}
	for _, stage := range r.Stages {
		if !stage.Passed {
			return false
		}
	}
	return true
}

func Run(ctx context.Context, workspace *repo.Workspace, options Options) (Report, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return Report{}, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}
	if options.Timeout <= 0 {
		return Report{}, fmt.Errorf("timeout must be greater than zero")
	}
	if options.Port <= 0 || options.Port > 65535 {
		return Report{}, fmt.Errorf("port must be between 1 and 65535")
	}
	logDir, err := prepareLogDir(workspace.Root, options.LogDir)
	if err != nil {
		return Report{}, err
	}
	runID := strconv.FormatInt(time.Now().UnixNano(), 36)
	imageTag := "rebuildable-verify-" + runID
	testImageTag := ""
	containerName := "rebuildable-verify-" + runID
	testContainerName := containerName + "-test"
	report := Report{ImageTag: imageTag, LogDir: logDir, Stages: make([]StageResult, 0, 4)}

	defer func() {
		cleanupDocker([]string{containerName, testContainerName}, []string{imageTag, testImageTag})
	}()

	build := runStage(ctx, options.Timeout, "build", filepath.Join(logDir, "build.log"), func(stageCtx context.Context, log io.Writer) (int, error) {
		return runCommand(stageCtx, log, "docker", "build", "--no-cache", "--tag", imageTag, "--file", filepath.Join(workspace.Root, "Dockerfile"), workspace.Root)
	})
	report.Stages = append(report.Stages, build)
	if !build.Passed {
		return finishReport(report, options), nil
	}

	if hasTestStage(workspace) {
		testImageTag = "rebuildable-verify-test-" + runID
		report.TestImageTag = testImageTag
		testBuild := runStage(ctx, options.Timeout, "test-build", filepath.Join(logDir, "test-build.log"), func(stageCtx context.Context, log io.Writer) (int, error) {
			return runCommand(stageCtx, log, "docker", "build", "--no-cache", "--target", "rebuildable-test", "--tag", testImageTag, "--file", filepath.Join(workspace.Root, "Dockerfile"), workspace.Root)
		})
		report.Stages = append(report.Stages, testBuild)
		if !testBuild.Passed {
			return finishReport(report, options), nil
		}
	}

	run, containerID, err := runContainerStage(ctx, workspace, options, imageTag, containerName, logDir)
	report.Stages = append(report.Stages, run)
	if err != nil {
		return report, err
	}
	if !run.Passed {
		report.Stages = append(report.Stages, StageResult{Name: "test", Error: "skipped because the run stage failed"})
		return finishReport(report, options), nil
	}

	test := runTestStage(ctx, workspace, options, imageTag, testImageTag, containerID, testContainerName, logDir)
	report.Stages = append(report.Stages, test)
	return finishReport(report, options), nil
}

func finishReport(report Report, options Options) Report {
	if report.Passed() && !options.KeepLogs {
		if report.LogDir != "" {
			_ = os.RemoveAll(report.LogDir)
		}
		report.LogDir = ""
		for index := range report.Stages {
			report.Stages[index].LogPath = ""
		}
	}
	return report
}

func prepareLogDir(root, requested string) (string, error) {
	if requested != "" {
		if filepath.IsAbs(requested) {
			return "", fmt.Errorf("log directory must be relative to the workspace")
		}
		clean := filepath.Clean(filepath.FromSlash(requested))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("log directory must remain inside the workspace")
		}
		requested = filepath.Join(root, clean)
	} else {
		requested = filepath.Join(root, ".rebuildable", "logs", strconv.FormatInt(time.Now().UnixNano(), 36))
	}
	if err := rejectSymlinkComponents(root, requested); err != nil {
		return "", err
	}
	if err := os.MkdirAll(requested, 0o755); err != nil {
		return "", fmt.Errorf("create verify log directory: %w", err)
	}
	return requested, nil
}

func rejectSymlinkComponents(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must remain inside the workspace")
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return fmt.Errorf("inspect log directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("log directory cannot contain symlink components")
		}
	}
	return nil
}

func runStage(ctx context.Context, timeout time.Duration, name, logPath string, run func(context.Context, io.Writer) (int, error)) StageResult {
	started := time.Now()
	stage := StageResult{Name: name, LogPath: logPath}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		stage.Error = err.Error()
		stage.Duration = time.Since(started)
		return stage
	}
	defer logFile.Close()
	stageCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	exitCode, runErr := run(stageCtx, logFile)
	stage.ExitCode = exitCode
	stage.Duration = time.Since(started)
	if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
		stage.TimedOut = true
		stage.Error = "stage timed out"
		return stage
	}
	if runErr != nil {
		stage.Error = runErr.Error()
		return stage
	}
	stage.Passed = exitCode == 0
	if !stage.Passed {
		stage.Error = fmt.Sprintf("command exited with status %d", exitCode)
	}
	return stage
}

func runCommand(ctx context.Context, log io.Writer, name string, args ...string) (int, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = log
	command.Stderr = log
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

func runContainerStage(ctx context.Context, workspace *repo.Workspace, options Options, imageTag, containerName, logDir string) (StageResult, string, error) {
	started := time.Now()
	stage := StageResult{Name: "run", LogPath: filepath.Join(logDir, "run.log")}
	logFile, err := os.OpenFile(stage.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		stage.Error = err.Error()
		return stage, "", nil
	}
	defer logFile.Close()
	stageCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	portMapping := fmt.Sprintf("%d:%d", options.Port, options.Port)
	exposesPort := dockerfileExposesPort(workspace, options.Port)
	createArgs := []string{"create", "--name", containerName}
	if exposesPort {
		createArgs = append(createArgs, "--publish", portMapping)
	}
	createArgs = append(createArgs, imageTag)
	var createOutput strings.Builder
	exitCode, commandErr := runCommandCapture(stageCtx, io.MultiWriter(logFile, &createOutput), "docker", createArgs...)
	if commandErr != nil || exitCode != 0 {
		stage.ExitCode = exitCode
		if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
			stage.TimedOut = true
			stage.Error = "stage timed out"
		} else {
			stage.Error = commandError(commandErr, exitCode, "docker create")
		}
		stage.Duration = time.Since(started)
		return stage, "", nil
	}
	containerID := strings.TrimSpace(createOutput.String())
	if containerID == "" {
		stage.Error = "docker create returned no container ID"
		stage.Duration = time.Since(started)
		return stage, "", nil
	}

	exitCode, commandErr = runCommand(stageCtx, logFile, "docker", "start", containerID)
	if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
		stage.TimedOut = true
		stage.Error = "stage timed out"
		stage.Duration = time.Since(started)
		return stage, containerID, nil
	}
	if commandErr != nil || exitCode != 0 {
		stage.ExitCode = exitCode
		if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
			stage.TimedOut = true
			stage.Error = "stage timed out"
		} else {
			stage.Error = commandError(commandErr, exitCode, "docker start")
		}
		stage.Duration = time.Since(started)
		return stage, containerID, nil
	}

	select {
	case <-stageCtx.Done():
		stage.TimedOut = true
		stage.Error = "stage timed out"
		stage.Duration = time.Since(started)
		return stage, containerID, nil
	case <-time.After(250 * time.Millisecond):
	}
	running, exitStatus, inspectErr := inspectContainer(stageCtx, containerID, logFile)
	if inspectErr != nil {
		if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
			stage.TimedOut = true
			stage.Error = "stage timed out"
		} else {
			stage.Error = inspectErr.Error()
		}
		stage.Duration = time.Since(started)
		return stage, containerID, nil
	}
	if running {
		if exposesPort {
			if err := waitForHTTP(stageCtx, options.Port); err != nil {
				if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
					stage.TimedOut = true
					stage.Error = "stage timed out"
				} else {
					stage.Error = err.Error()
				}
				stage.Duration = time.Since(started)
				return stage, containerID, nil
			}
		}
	} else if exitStatus != 0 {
		stage.ExitCode = exitStatus
		stage.Error = fmt.Sprintf("container exited with status %d", exitStatus)
		stage.Duration = time.Since(started)
		return stage, containerID, nil
	}
	stage.Passed = true
	stage.ExitCode = exitStatus
	stage.Duration = time.Since(started)
	return stage, containerID, nil
}

func runTestStage(ctx context.Context, workspace *repo.Workspace, options Options, imageTag, testImageTag, containerID, testContainerName, logDir string) StageResult {
	started := time.Now()
	stage := StageResult{Name: "test", LogPath: filepath.Join(logDir, "test.log")}
	logFile, err := os.OpenFile(stage.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		stage.Error = err.Error()
		return stage
	}
	defer logFile.Close()
	command, err := testCommand(workspace)
	if err != nil {
		stage.Error = err.Error()
		stage.Duration = time.Since(started)
		return stage
	}
	stageCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	var exitCode int
	if testImageTag != "" {
		args := append([]string{"run", "--name", testContainerName, "--network", "none", "--entrypoint", "", testImageTag}, command...)
		exitCode, err = runCommand(stageCtx, logFile, "docker", args...)
	} else if containerRunning(stageCtx, containerID) {
		exitCode, err = runCommand(stageCtx, logFile, "docker", append([]string{"exec", containerID}, command...)...)
	} else {
		exitCode, err = runCommand(stageCtx, logFile, "docker", append([]string{"run", "--name", testContainerName, "--network", "none", "--entrypoint", "", imageTag}, command...)...)
	}
	stage.ExitCode = exitCode
	stage.Duration = time.Since(started)
	if errors.Is(stageCtx.Err(), context.DeadlineExceeded) {
		stage.TimedOut = true
		stage.Error = "stage timed out"
		return stage
	}
	if err != nil {
		stage.Error = err.Error()
		return stage
	}
	stage.Passed = exitCode == 0
	if !stage.Passed {
		stage.Error = fmt.Sprintf("test command exited with status %d", exitCode)
	}
	return stage
}

func hasTestStage(workspace *repo.Workspace) bool {
	if !workspace.HasFile("Dockerfile") {
		return false
	}
	contents, err := workspace.Read("Dockerfile")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && strings.EqualFold(fields[0], "FROM") && strings.EqualFold(fields[len(fields)-2], "AS") && strings.EqualFold(fields[len(fields)-1], "rebuildable-test") {
			return true
		}
	}
	return false
}

func dockerfileExposesPort(workspace *repo.Workspace, port int) bool {
	if !workspace.HasFile("Dockerfile") {
		return false
	}
	contents, err := workspace.Read("Dockerfile")
	if err != nil {
		return false
	}
	for _, rawLine := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(strings.TrimSpace(rawLine))
		if len(fields) < 2 || !strings.EqualFold(fields[0], "EXPOSE") {
			continue
		}
		for _, field := range fields[1:] {
			field = strings.TrimSuffix(field, "/tcp")
			field = strings.TrimSuffix(field, "/udp")
			if value, err := strconv.Atoi(field); err == nil && value == port {
				return true
			}
		}
	}
	return false
}

func testCommand(workspace *repo.Workspace) ([]string, error) {
	if len(workspace.Ecosystems) != 1 {
		return nil, fmt.Errorf("verify needs exactly one detected ecosystem to choose a test command")
	}
	switch workspace.Ecosystems[0] {
	case repo.EcosystemGo:
		return []string{"go", "test", "./..."}, nil
	case repo.EcosystemNode:
		return []string{"npm", "test"}, nil
	case repo.EcosystemPython:
		return []string{"python", "-m", "pytest"}, nil
	default:
		return nil, fmt.Errorf("unsupported test ecosystem %q", workspace.Ecosystems[0])
	}
}

func runCommandCapture(ctx context.Context, output io.Writer, name string, args ...string) (int, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

func commandError(err error, exitCode int, name string) string {
	if err != nil {
		return fmt.Sprintf("%s: %v", name, err)
	}
	return fmt.Sprintf("%s exited with status %d", name, exitCode)
}

func inspectContainer(ctx context.Context, containerID string, log io.Writer) (bool, int, error) {
	var output strings.Builder
	exitCode, err := runCommandCapture(ctx, io.MultiWriter(log, &output), "docker", "inspect", "--format", "{{.State.Running}}|{{.State.ExitCode}}", containerID)
	if err != nil {
		return false, 0, err
	}
	if exitCode != 0 {
		return false, exitCode, fmt.Errorf("docker inspect exited with status %d", exitCode)
	}
	fields := strings.Split(strings.TrimSpace(output.String()), "|")
	if len(fields) != 2 {
		return false, 0, fmt.Errorf("unexpected docker inspect output")
	}
	running := fields[0] == "true"
	status, err := strconv.Atoi(fields[1])
	if err != nil {
		return running, 0, fmt.Errorf("parse container exit status: %w", err)
	}
	return running, status, nil
}

func containerRunning(ctx context.Context, containerID string) bool {
	if containerID == "" {
		return false
	}
	command := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Running}}", containerID)
	output, err := command.Output()
	return err == nil && strings.TrimSpace(string(output)) == "true"
}

func waitForHTTP(ctx context.Context, port int) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port), nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("container did not respond on port %d", port)
		case <-ticker.C:
		}
	}
}

func cleanupDocker(containerNames []string, imageTags []string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, containerName := range containerNames {
		if containerName != "" {
			_ = exec.CommandContext(cleanupCtx, "docker", "rm", "--force", containerName).Run()
		}
	}
	for _, imageTag := range imageTags {
		if imageTag != "" {
			_ = exec.CommandContext(cleanupCtx, "docker", "rmi", "--force", imageTag).Run()
		}
	}
}
