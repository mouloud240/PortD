// Package config loads PortD's process configuration.
package config

import "os"

const defaultHTTPAddr = "127.0.0.1:8080"

// Config contains process-level settings.
type Config struct {
	HTTPAddr string
	BaseURL  string
}

// Load returns configuration from environment variables with safe defaults.
func Load() Config {
	address := os.Getenv("PORTD_HTTP_ADDR")
	if address == "" {
		address = defaultHTTPAddr
	}

	baseURL := os.Getenv("BASE_URL")
	return Config{HTTPAddr: address, BaseURL: baseURL}
}
