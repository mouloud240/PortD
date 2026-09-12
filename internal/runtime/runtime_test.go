package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"slices"
	"testing"
)

func TestFakeStarterRecordsProject(t *testing.T) {
	t.Parallel()

	starter := &FakeStarter{Err: errors.New("failed")}
	project := Project{Directory: "projects/demo", Executable: "./start.sh"}
	_, err := starter.Start(context.Background(), project)
	if !errors.Is(err, starter.Err) {
		t.Fatalf("error = %v, want fake error", err)
	}
	if len(starter.Requests) != 1 ||
		starter.Requests[0].Directory != project.Directory ||
		starter.Requests[0].Executable != project.Executable ||
		!slices.Equal(starter.Requests[0].Arguments, project.Arguments) {
		t.Fatalf("requests = %+v, want %+v", starter.Requests, project)
	}
}

func TestResolveExecutable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "start.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "start.bat"), []byte("@echo off\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	defaultFile := "start.sh"
	if stdruntime.GOOS == "windows" {
		defaultFile = "start.bat"
	}
	tests := []struct {
		name    string
		config  string
		want    string
		wantErr error
		errAny  bool
	}{
		{name: "platform default script", want: defaultFile},
		{name: "existing explicit script", config: "./start.sh", want: defaultFile},
		{name: "missing script", config: "missing.sh", wantErr: ErrStartupMissing},
		{name: "path traversal", config: "../start.sh", errAny: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveExecutable(dir, tt.config)
			if tt.errAny {
				if err == nil {
					t.Fatal("error = nil, want validation error")
				}
				return
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("executable = %q, want %q", got, tt.want)
			}
		})
	}
}
