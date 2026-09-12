//go:build !windows

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerStartsAndStopsProcessGroup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := filepath.Join(dir, "start.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntrap 'exit 0' TERM\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	manager := NewManager()
	status, err := manager.Start(context.Background(), ManagedProject{
		ID:         "project-1",
		Directory:  dir,
		Executable: "start.sh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateRunning || status.PID == 0 {
		t.Fatalf("status = %+v, want running process", status)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.Stop(stopCtx, "project-1"); err != nil {
		t.Fatal(err)
	}
	if got := manager.Status("project-1").State; got != StateStopped {
		t.Fatalf("state = %q, want %q", got, StateStopped)
	}
}
