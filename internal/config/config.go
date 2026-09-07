// Package config loads PortD's process configuration.
package config

import (
	"os"
	"strings"
)

const (
	defaultHTTPAddr      = "127.0.0.1:8080"
	defaultDBPath        = "tmp/portd.db"
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin-password"
)

// Config contains process-level settings.
type Config struct {
	HTTPAddr      string
	BaseURL       string
	DBPath        string
	AdminUsername string
	AdminPassword string
}

// Load returns configuration from environment variables with safe defaults.
func Load() Config {
	address := os.Getenv("PORTD_HTTP_ADDR")
	if address == "" {
		address = defaultHTTPAddr
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://" + address
	}
	databasePath := os.Getenv("PORTD_DB_PATH")
	if databasePath == "" {
		databasePath = defaultDBPath
	}
	adminUsername := os.Getenv("PORTD_ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = defaultAdminUsername
	}
	adminPassword := os.Getenv("PORTD_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = defaultAdminPassword
	}
	return Config{HTTPAddr: address, BaseURL: baseURL, DBPath: databasePath, AdminUsername: adminUsername, AdminPassword: adminPassword}
}
