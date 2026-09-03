package app

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/db/generated"
	internsvc "github.com/portd/internal/interns"
	_ "modernc.org/sqlite"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	queries := db.New(testDB(t))
	NewHandler(auth.NewService(queries, "admin", "admin-password"), internsvc.NewService(queries)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "ok\n" {
		t.Fatalf("body = %q, want %q", body, "ok\\n")
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
