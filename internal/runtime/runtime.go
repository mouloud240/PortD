// Package runtime contains process-startup seams for project reconciliation.
package runtime

import (
	"context"
	"errors"
	"os/exec"
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
