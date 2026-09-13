// Package config loads PortD's process configuration.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"github.com/joho/godotenv"
)

const (
	defaultHTTPAddr      = "127.0.0.1:8080"
	defaultDBPath        = "tmp/portd.db"
	defaultProjectsDir   = "~/dev/portd-projects"
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin-password"
)

// Config contains process-level settings.
type Config struct {
	HTTPAddr      string
	BaseURL       string
	DBPath        string
	ProjectsDir   string
	AdminUsername string
	AdminPassword string
}

// Load returns configuration from environment variables with safe defaults.
func Load() Config {
	err:=godotenv.Load(".env")
	if err!=nil{
		panic(err.Error())
	}
	address := os.Getenv("PORTD_HTTP_ADDR")
	if address == "" {
		address = defaultHTTPAddr
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://" + address
	}
	databasePath := expandHome(os.Getenv("PORTD_DB_PATH"))
	if databasePath == "" {
		databasePath = defaultDBPath
	}
	projectsDir := strings.TrimRight(strings.TrimSpace(expandHome(os.Getenv("PORTD_PROJECTS_DIR"))), "/")
	if projectsDir == "" {
		projectsDir = defaultProjectsDir
	}
	adminUsername := os.Getenv("PORTD_ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = defaultAdminUsername
	}
	adminPassword := os.Getenv("PORTD_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = defaultAdminPassword
	}
	return Config{HTTPAddr: address, BaseURL: baseURL, DBPath: databasePath, ProjectsDir: projectsDir, AdminUsername: adminUsername, AdminPassword: adminPassword}
}

// expandHome resolves a leading ~/ in env-supplied paths. Quoted values in
// .env files never see shell tilde expansion, so without this PortD would
// create a literal ~ directory.
func expandHome(path string) string {
	if path == "~" {
		path = "~/"
	}
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}
