package httperr

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWritesHTTPError(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	Handle(func(http.ResponseWriter, *http.Request) error {
		return NotFound("Intern not found", errors.New("missing"))
	}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/interns/x", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if !strings.Contains(response.Body.String(), "Intern not found") {
		t.Fatalf("body = %q, want safe message", response.Body.String())
	}
}

func TestHandleHidesUnexpectedError(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	Handle(func(http.ResponseWriter, *http.Request) error {
		return errors.New("database password leaked")
	}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), "database password") {
		t.Fatal("response exposes internal error")
	}
}

func TestHandleDoesNotWriteAfterHandlerResponse(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	Handle(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusTeapot)
		return errors.New("late error")
	}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTeapot)
	}
}
