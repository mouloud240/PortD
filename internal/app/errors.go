package app

import (
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
)

// Handler is an HTTP handler that reports failures to the shared error writer.
type Handler func(http.ResponseWriter, *http.Request) error

// HTTPError describes a safe HTTP response and preserves its internal cause.
type HTTPError struct {
	Status  int
	Message string
	Err     error
}

func (e *HTTPError) Error() string { return e.Message }

func (e *HTTPError) Unwrap() error { return e.Err }

func httpError(status int, message string, err error) error {
	return &HTTPError{Status: status, Message: message, Err: err}
}

func BadRequest(message string, err error) error {
	return httpError(http.StatusBadRequest, message, err)
}
func Unauthorized(message string, err error) error {
	return httpError(http.StatusUnauthorized, message, err)
}
func Forbidden(message string, err error) error { return httpError(http.StatusForbidden, message, err) }
func NotFound(message string, err error) error  { return httpError(http.StatusNotFound, message, err) }
func Conflict(message string, err error) error  { return httpError(http.StatusConflict, message, err) }

// Handle converts an error-returning handler into a standard HTTP handler.
func Handle(handler Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &responseWriter{ResponseWriter: w}
		if err := handler(tracked, r); err != nil && !tracked.wrote {
			writeError(tracked, err)
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *responseWriter) WriteHeader(status int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func writeError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "Internal server error"
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status, message = httpErr.Status, httpErr.Message
	} else {
		slog.Error("request failed", "error", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<!doctype html><html><body><h1>%d</h1><p>%s</p></body></html>", status, html.EscapeString(message))
}
