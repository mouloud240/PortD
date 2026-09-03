package config

import "testing"

func TestLoadUsesDefaultHTTPAddress(t *testing.T) {
	t.Setenv("PORTD_HTTP_ADDR", "")

	if got := Load().HTTPAddr; got != defaultHTTPAddr {
		t.Fatalf("HTTPAddr = %q, want %q", got, defaultHTTPAddr)
	}
}

func TestLoadReadsHTTPAddress(t *testing.T) {
	t.Setenv("PORTD_HTTP_ADDR", "127.0.0.1:9090")

	if got := Load().HTTPAddr; got != "127.0.0.1:9090" {
		t.Fatalf("HTTPAddr = %q, want %q", got, "127.0.0.1:9090")
	}
}
