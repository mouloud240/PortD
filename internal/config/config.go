// Package config loads PortD's process configuration.
package config

import "os"

const (
	defaultHTTPAddr = "127.0.0.1:8080"
	defaultDBPath   = "tmp/portd.db"
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

	baseURL := os.Getenv("BASE_URL")
	databasePath := os.Getenv("PORTD_DB_PATH")
	if databasePath == "" {
		databasePath = defaultDBPath
	}
	return Config{HTTPAddr: address, BaseURL: baseURL, DBPath: databasePath, AdminUsername: os.Getenv("PORTD_ADMIN_USERNAME"), AdminPassword: os.Getenv("PORTD_ADMIN_PASSWORD")}
}
