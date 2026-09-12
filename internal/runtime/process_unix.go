//go:build !windows

package runtime

import (
	"context"
	"os/exec"
	"syscall"
)

func commandForPlatform(ctx context.Context, directory, executable string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, filepathForExecution(directory, executable)), nil
}

func filepathForExecution(directory, executable string) string {
	return "./" + executable
}

func configureProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func terminateProcess(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

func forceTerminateProcess(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
