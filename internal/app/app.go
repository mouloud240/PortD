// Package app wires PortD's transport-level dependencies.
package app

import (
	"net/http"
)

// NewHandler returns the root HTTP handler for PortD.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", Handle(healthz))
	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte("ok\n"))
	return err
}
