package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Scaffold describes the stable project artifacts PortD creates.
// Name, Description, ports, and URLs feed the generated README; the
// directory and scripts only need Directory, Slug, and StartupCommand.
type Scaffold struct {
	Directory      string
	Slug           string
	Name           string
	Description    string
	StartupCommand string
	AccessMode     string
	Ports          []PortRef
	MainPort       int64
	PublicURL      string
	DirectURL      string
}

// PortRef is one assigned port and its role for the generated README.
type PortRef struct {
	Port int64
	Role string
	Main bool
}

// Scaffolder creates project artifacts around project creation.
type Scaffolder interface {
	// Create makes the directory and startup scripts. It runs before the
	// project row exists and must stay idempotent.
	Create(context.Context, Scaffold) error
	// WriteReadme regenerates the project README with full details. It
	// runs after the row and ports exist and overwrites on each call.
	WriteReadme(context.Context, Scaffold) error
}

// FileScaffolder creates an idempotent directory, startup scripts, and README.
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
		slog.Error("project directory create failed", "slug", scaffold.Slug, "dir", scaffold.Directory, "error", err)
		return err
	}
	slog.Info("project directory ready", "slug", scaffold.Slug, "dir", scaffold.Directory)
	startup := filepath.Join(scaffold.Directory, "start.sh")
	if err := writeIfAbsent(startup, "#!/bin/sh\nset -eu\n", 0o755); err != nil {
		return err
	}
	windowsStartup := filepath.Join(scaffold.Directory, "start.bat")
	if err := writeIfAbsent(windowsStartup, "@echo off\r\n", 0o755); err != nil {
		return err
	}
	return nil
}

// WriteReadme regenerates the project README with the project details,
// PortD instructions, startup files, ports, and access modes.
func (s FileScaffolder) WriteReadme(ctx context.Context, scaffold Scaffold) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if scaffold.Directory == "" {
		return fmt.Errorf("scaffold directory is required")
	}
	return os.WriteFile(filepath.Join(scaffold.Directory, "README.md"), []byte(projectReadme(scaffold)), 0o644)
}

func projectReadme(scaffold Scaffold) string {
	var out strings.Builder
	name := scaffold.Name
	if name == "" {
		name = scaffold.Slug
	}
	out.WriteString("# " + name + "\n\n")
	if scaffold.Description != "" {
		out.WriteString(scaffold.Description + "\n\n")
	}
	out.WriteString("Managed by PortD. Do not rename this directory: the hub maps it to the `" + scaffold.Slug + "` project.\n")
	out.WriteString("\n## Run it\n\n")
	out.WriteString("PortD starts and stops this project from its detail page in the hub. ")
	out.WriteString("It runs `" + scaffold.StartupCommand + "` (`start.sh` on Unix, `start.bat` on Windows), ")
	out.WriteString("so put your launch command in the file for your platform.\n")
	out.WriteString("\n## Startup files\n\n")
	out.WriteString("- `start.sh` — Unix entry point, executed as `./start.sh`.\n")
	out.WriteString("- `start.bat` — Windows entry point, executed via `cmd /c`.\n")
	out.WriteString("- Keep both files present; PortD picks the one for its platform. ")
	out.WriteString("To run a different file, change the execution file under Runtime on the project detail page.\n")
	out.WriteString("\n### Windows tips\n\n")
	out.WriteString("- Keep one blocking server command at the end of the bat file so PortD can track and stop it.\n")
	out.WriteString("- Avoid `start ... cmd /k`, `/min` or `pause`; those keep processes alive after Stop.\n")
	out.WriteString("- To run a background helper, use `start /b \"\"` inside the same console, or split backend/frontend into two PortD projects.\n")
	out.WriteString("\n## Ports\n\n")
	if len(scaffold.Ports) == 0 {
		out.WriteString("No ports assigned yet.\n")
	} else {
		for _, port := range scaffold.Ports {
			line := "- Port " + strconv.FormatInt(port.Port, 10)
			if port.Role != "" {
				line += " (" + port.Role + ")"
			}
			if port.Main {
				line += " — main port, serves the UI and backs the public URL"
			}
			out.WriteString(line + "\n")
		}
		if scaffold.MainPort != 0 {
			out.WriteString("\nYour app must bind `localhost:" + strconv.FormatInt(scaffold.MainPort, 10) + "`.\n")
		}
	}
	out.WriteString("\n## Access\n\n")
	if scaffold.PublicURL != "" {
		out.WriteString("- Proxied: " + scaffold.PublicURL + " — served under the hub path. ")
		out.WriteString("Client-side routing and root-relative assets may need a base path; see the framework quickstarts on the project page.\n")
	}
	if scaffold.DirectURL != "" {
		out.WriteString("- Direct: " + scaffold.DirectURL + " — the app server straight on its main port, no app changes required.\n")
	}
	out.WriteString("\n## Layout\n\n")
	out.WriteString("- `start.sh`, `start.bat` — entry points, yours to edit.\n")
	out.WriteString("- `README.md` — this file; regenerated by PortD, local edits may be overwritten.\n")
	return out.String()
}

func writeIfAbsent(path, content string, mode fs.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}
