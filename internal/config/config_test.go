package config

import "testing"

func TestLoadUsesDefaultHTTPAddress(t *testing.T) {
	t.Setenv("PORTD_HTTP_ADDR", "")
	t.Setenv("BASE_URL", "")

	cfg := Load()
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
	if want := "http://" + defaultHTTPAddr; cfg.BaseURL != want {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, want)
	}
}

func TestLoadReadsBaseURL(t *testing.T) {
	t.Setenv("BASE_URL", "https://portd.example.test/")

	if got := Load().BaseURL; got != "https://portd.example.test" {
		t.Fatalf("BaseURL = %q, want %q", got, "https://portd.example.test")
	}
}

func TestLoadReadsHTTPAddress(t *testing.T) {
	t.Setenv("PORTD_HTTP_ADDR", "127.0.0.1:9090")

	if got := Load().HTTPAddr; got != "127.0.0.1:9090" {
		t.Fatalf("HTTPAddr = %q, want %q", got, "127.0.0.1:9090")
	}
}

func TestLoadUsesDefaultAdmin(t *testing.T) {
	t.Setenv("PORTD_ADMIN_USERNAME", "")
	t.Setenv("PORTD_ADMIN_PASSWORD", "")
	cfg := Load()
	if cfg.AdminUsername != defaultAdminUsername || cfg.AdminPassword != defaultAdminPassword {
		t.Fatalf("admin defaults = %q/%q", cfg.AdminUsername, cfg.AdminPassword)
	}
}
