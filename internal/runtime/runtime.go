// Package runtime contains process-startup seams for project reconciliation.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Project describes the persisted fields needed to start a project.
type Project struct {
	Directory  string
	Executable string
	Arguments  []string
}

// Result describes a completed startup attempt.
type Result struct {
	Output string
}

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateFailed   State = "failed"
)

var (
	ErrAlreadyRunning = errors.New("runtime: project is already running")
	ErrNotRunning     = errors.New("runtime: project is not running")
	ErrStartupMissing = errors.New("runtime: startup file is missing")
)

type Status struct {
	State State
	PID   int
	Error string
	File  string
}

// Starter starts a project without exposing process details to callers.
type Starter interface {
	Start(context.Context, Project) (Result, error)
}

// CommandStarter runs an executable with arguments in its project directory.
type CommandStarter struct{}

func (CommandStarter) Start(ctx context.Context, project Project) (Result, error) {
	if project.Executable == "" {
		return Result{}, errors.New("runtime: executable is required")
	}
	cmd := exec.CommandContext(ctx, project.Executable, project.Arguments...)
	cmd.Dir = project.Directory
	output, err := cmd.CombinedOutput()
	return Result{Output: string(output)}, err
}

// FakeStarter records startup requests for deterministic tests.
type FakeStarter struct {
	Requests []Project
	Result   Result
	Err      error
}

func (f *FakeStarter) Start(_ context.Context, project Project) (Result, error) {
	f.Requests = append(f.Requests, project)
	return f.Result, f.Err
}

type ManagedProject struct {
	ID         string
	Directory  string
	Executable string
}

type process struct {
	cmd    *exec.Cmd
	status Status
	done   chan struct{}
}

type Manager struct {
	mu        sync.Mutex
	processes map[string]*process
}

func NewManager() *Manager {
	return &Manager{processes: make(map[string]*process)}
}

func (m *Manager) Start(ctx context.Context, project ManagedProject) (Status, error) {
	if project.ID == "" || project.Directory == "" {
		return Status{State: StateFailed}, errors.New("runtime: project id and directory are required")
	}
	executable, err := ResolveExecutable(project.Directory, project.Executable)

	if err != nil {
		slog.Log(context.Background(), slog.LevelError, "resolve executable", "error", err, "project", project.ID)
		return Status{State: StateFailed, File: project.Executable}, err
	}

	m.mu.Lock()
	if current := m.processes[project.ID]; current != nil && current.cmd.Process != nil {
		m.mu.Unlock()
		return current.status, ErrAlreadyRunning
	}
	runCtx := context.WithoutCancel(ctx)
	command, err := commandForPlatform(runCtx, project.Directory, executable)
	if err != nil {
		m.mu.Unlock()
		return Status{State: StateFailed, File: executable}, err
	}
	command.Dir = project.Directory
	if err := configureProcess(command); err != nil {
		m.mu.Unlock()
		return Status{State: StateFailed, File: executable}, err
	}
	if err := command.Start(); err != nil {
		m.mu.Unlock()
		slog.Log(context.Background(), slog.LevelError, "resolve executable", "error", err, "project", project.ID)

		return Status{State: StateFailed, File: executable}, fmt.Errorf("runtime: start %s: %w", executable, err)
	}
	current := &process{
		cmd:    command,
		status: Status{State: StateRunning, PID: command.Process.Pid, File: executable},
		done:   make(chan struct{}),
	}
	m.processes[project.ID] = current
	m.mu.Unlock()

	go m.wait(project.ID, current)
	return current.status, nil
}

func (m *Manager) Stop(ctx context.Context, projectID string) error {
	m.mu.Lock()
	current := m.processes[projectID]
	if current == nil || current.cmd.Process == nil {
		m.mu.Unlock()
		return ErrNotRunning
	}
	current.status.State = StateStopping
	err := terminateProcess(current.cmd)
	m.mu.Unlock()
	if err != nil {
		return fmt.Errorf("runtime: stop project %s: %w", projectID, err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-current.done:
		return nil
	case <-time.After(2 * time.Second):
		if err := forceTerminateProcess(current.cmd); err != nil {
			return fmt.Errorf("runtime: force stop project %s: %w", projectID, err)
		}
		return nil
	}
}

func (m *Manager) Status(projectID string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.processes[projectID]; current != nil {
		return current.status
	}
	return Status{State: StateStopped}
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	ids := make([]string, 0, len(m.processes))
	for id, current := range m.processes {
		if current.cmd.Process != nil {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		if err := m.Stop(ctx, id); err != nil && !errors.Is(err, ErrNotRunning) {
			return err
		}
	}
	return nil
}

func (m *Manager) wait(projectID string, current *process) {
	err := current.cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.processes[projectID] != current {
		return
	}
	current.cmd.Process = nil
	if err != nil && current.status.State != StateStopping && !errors.Is(err, context.Canceled) {
		current.status.State = StateFailed
		current.status.Error = err.Error()
		close(current.done)
		return
	}
	current.status.State = StateStopped
	current.status.PID = 0
	close(current.done)
}

func ResolveExecutable(directory, configured string) (string, error) {
	candidate := configured
	normalized := filepath.Clean(candidate)
	if candidate == "" || normalized == "start.sh" || normalized == "start.bat" {
		candidate = "start.sh"
		if runtime.GOOS == "windows" {
			candidate = "start.bat"
		}
	}
	candidate = filepath.Clean(candidate)
	if filepath.IsAbs(candidate) || candidate == "." || candidate == ".." || strings.HasPrefix(candidate, ".."+string(filepath.Separator)) {
		return "", errors.New("runtime: startup file must be inside the project directory")
	}
	path := filepath.Join(directory, candidate)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: %s", ErrStartupMissing, candidate)
	}
	if err != nil {
		return "", fmt.Errorf("runtime: inspect startup file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("runtime: startup path is a directory: %s", candidate)
	}
	return candidate, nil
}
