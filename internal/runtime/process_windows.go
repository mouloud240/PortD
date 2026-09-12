//go:build windows

package runtime

import (
	"context"
	"os/exec"
)

func commandForPlatform(ctx context.Context, directory, executable string) (*exec.Cmd, error) {
	if len(executable) >= 4 && executable[len(executable)-4:] == ".bat" {
		return exec.CommandContext(ctx, "cmd", "/c", executable), nil
	}
	return exec.CommandContext(ctx, executable), nil
}

func configureProcess(_ *exec.Cmd) error {
	return nil
}

func terminateProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

func forceTerminateProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
