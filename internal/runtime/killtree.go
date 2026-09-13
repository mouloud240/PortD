package runtime

import (
	"errors"
	"os/exec"
	"strconv"
)

// runTaskkill is a seam so tests can stub the OS call.
var runTaskkill = func(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

// killTree kills a process and its children (/T covers the cmd.exe wrapper
// plus the app it spawned). A PID that is already gone counts as success;
// a missing taskkill binary uses fallback (single-PID kill).
func killTree(pid int, fallback func() error) error {
	err := runTaskkill(pid)
	if err == nil || taskkillGone(err) {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		// ponytail: taskkill absent (e.g. Nano Server); old single-PID behavior.
		return fallback()
	}
	return err
}

// taskkillGone reports taskkill's exit 128: the PID was already dead.
func taskkillGone(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 128
}
