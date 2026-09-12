package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// Scaffold describes the stable project artifacts PortD creates.
type Scaffold struct {
	Directory      string
	Slug           string
	StartupCommand string
	MainPort       int64
	PublicURL      string
}

// Scaffolder creates project artifacts before runtime startup.
type Scaffolder interface {
	Create(context.Context, Scaffold) error
}

// FileScaffolder creates an idempotent directory, startup script, and README.
type FileScaffolder struct {
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (s FileScaffolder) Create(ctx context.Context, scaffold Scaffold) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if scaffold.Directory == "" || !slugPattern.MatchString(scaffold.Slug) {
		return fmt.Errorf("scaffold directory and slug are required")
	}
	if err := os.MkdirAll(scaffold.Directory, 0o755); err != nil {
		return err
	}
	startup := filepath.Join(scaffold.Directory, "start.sh")
	if err := writeIfAbsent(startup, "#!/bin/sh\nset -eu\n", 0o755); err != nil {
		return err
	}
	windowsStartup := filepath.Join(scaffold.Directory, "start.bat")
	if err := writeIfAbsent(windowsStartup, "@echo off\r\n", 0o755); err != nil {
		return err
	}
	readme := filepath.Join(scaffold.Directory, "README.md")
	content := fmt.Sprintf("# %s\n\n- URL: %s\n- Bind: localhost:%d\n- Unix startup: `start.sh`\n- Windows startup: `start.bat`\n- Configured startup: `%s`\n", scaffold.Slug, scaffold.PublicURL, scaffold.MainPort, scaffold.StartupCommand)
	return writeIfAbsent(readme, content, 0o644)
}

func writeIfAbsent(path, content string, mode fs.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}
