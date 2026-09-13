package runtime

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// TestHelperExit128 re-execs as a child that exits 128, giving us a real
// *exec.ExitError without depending on a platform shell.
func TestHelperExit128(t *testing.T) {
	if os.Getenv("PORTD_TEST_HELPER") != "exit128" {
		return
	}
	os.Exit(128)
}

func exit128(t *testing.T) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperExit128")
	cmd.Env = append(os.Environ(), "PORTD_TEST_HELPER=exit128")
	return cmd.Run()
}

func TestKillTree(t *testing.T) {
	if taskkillGone(nil) || taskkillGone(errors.New("boom")) {
		t.Fatalf("taskkillGone should only match exit 128")
	}
	if err := exit128(t); !taskkillGone(err) {
		t.Fatalf("taskkillGone(%v) = false, want true", err)
	}

	old := runTaskkill
	defer func() { runTaskkill = old }()
	stub := func(err error) { runTaskkill = func(int) error { return err } }
	noop := func() error { return nil }

	stub(nil)
	if err := killTree(1234, noop); err != nil {
		t.Fatalf("killTree(nil) = %v, want nil", err)
	}
	stub(exit128(t))
	if err := killTree(1234, noop); err != nil {
		t.Fatalf("killTree(128) = %v, want nil", err)
	}
	boom := errors.New("boom")
	stub(boom)
	if err := killTree(1234, noop); !errors.Is(err, boom) {
		t.Fatalf("killTree(boom) = %v, want boom", err)
	}
	called := false
	stub(exec.ErrNotFound)
	if err := killTree(1234, func() error { called = true; return nil }); err != nil || !called {
		t.Fatalf("killTree(notfound) = %v, fallback %v; want nil + fallback", err, called)
	}
}
