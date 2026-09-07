package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/db/generated"
	internsvc "github.com/portd/internal/interns"
	projectsvc "github.com/portd/internal/projects"
	_ "modernc.org/sqlite"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	database := testDB(t)
	queries := db.New(database)
	testHandler(t, database, auth.NewService(queries, "admin", "admin-password")).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "ok\n" {
		t.Fatalf("body = %q, want %q", body, "ok\\n")
	}
}

func TestLoginPage(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()

	database := testDB(t)
	queries := db.New(database)
	testHandler(t, database, auth.NewService(queries, "admin", "admin-password")).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "/static/app.css") {
		t.Errorf("login page should link /static/app.css")
	}
	if !strings.Contains(body, "bg-brand-ground") {
		t.Errorf("login page should contain Tailwind class bg-brand-ground")
	}
	if !strings.Contains(body, "Toggle password visibility") {
		t.Errorf("login page should contain the password visibility toggle")
	}
}

func TestPlaceholderPage(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	initDB(t, database)
	queries := db.New(database)
	authService := auth.NewService(queries, "admin", "admin-password")
	session, err := authService.Login(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	response := httptest.NewRecorder()

	NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "bg-brand-ground") {
		t.Errorf("dashboard should contain Tailwind class bg-brand-ground")
	}
	if !strings.Contains(body, "text-brand-navy") {
		t.Errorf("dashboard should contain Tailwind class text-brand-navy")
	}
}

func TestInternsListPage(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	initDB(t, database)
	queries := db.New(database)
	authService := auth.NewService(queries, "admin", "admin-password")
	session, err := authService.Login(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/interns", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	response := httptest.NewRecorder()

	NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "text-brand-navy") {
		t.Errorf("interns list should contain Tailwind class text-brand-navy")
	}
	if !strings.Contains(body, "bg-brand-blue") {
		t.Errorf("interns list should contain Tailwind class bg-brand-blue")
	}
}

func TestInternNewPage(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	initDB(t, database)
	queries := db.New(database)
	authService := auth.NewService(queries, "admin", "admin-password")
	session, err := authService.Login(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/interns/new", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	response := httptest.NewRecorder()

	NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Save intern") {
		t.Errorf("form should contain Save intern button")
	}
	if !strings.Contains(body, "bg-brand-blue") {
		t.Errorf("form button should contain Tailwind class bg-brand-blue")
	}
	if !strings.Contains(body, "Toggle password visibility") {
		t.Errorf("new intern form should contain the password visibility toggle")
	}
	if !strings.Contains(body, `action="/interns"`) {
		t.Errorf("new intern form should post to /interns")
	}
}

func TestInternCreateValidationConflictAndNotFound(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	initDB(t, database)
	queries := db.New(database)
	authService := auth.NewService(queries, "admin", "admin-password")
	session, err := authService.Login(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	handler := NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries))
	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	invalid := post("/interns", url.Values{"full_name": {""}, "identifier": {""}, "password": {""}})
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid create status = %d, want %d", invalid.Code, http.StatusUnprocessableEntity)
	}

	// The form used to post back to /interns/new, which the router matches
	// as POST /interns/{id} with id="new". That must never create or
	// update anything.
	stale := post("/interns/new", url.Values{"full_name": {"Ghost"}, "identifier": {"ghost"}, "password": {"secret-123"}})
	if stale.Code != http.StatusNotFound {
		t.Fatalf("stale form target status = %d, want %d", stale.Code, http.StatusNotFound)
	}
	items, err := internsvc.NewService(queries).List(context.Background(), "", -1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("stale form target created %d interns, want 0", len(items))
	}

	valid := url.Values{"full_name": {"Amina"}, "identifier": {"amina"}, "email": {"amina@example.com"}, "password": {"secret-123"}}
	created := post("/interns", valid)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("valid create status = %d, want %d", created.Code, http.StatusSeeOther)
	}
	detail := httptest.NewRequest(http.MethodGet, created.Header().Get("Location"), nil)
	detail.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, detail)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", detailResponse.Code, http.StatusOK)
	}
	if !strings.Contains(detailResponse.Body.String(), "Deactivate account") {
		t.Errorf("edit form should contain the Deactivate account button")
	}
	duplicate := post("/interns", valid)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, want %d", duplicate.Code, http.StatusConflict)
	}
	if !strings.Contains(duplicate.Body.String(), "already taken") {
		t.Errorf("duplicate create should explain the conflict")
	}

	missing := httptest.NewRequest(http.MethodGet, "/interns/missing", nil)
	missing.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, missing)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing intern status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func initDB(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000001_initial_schema.up.sql", "000002_add_routes.up.sql", "000003_add_local_auth.up.sql"} {
		schema, err := os.ReadFile(filepath.Join("..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(string(schema)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
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

func testHandler(t *testing.T, database *sql.DB, authService *auth.Service) http.Handler {
	t.Helper()
	queries := db.New(database)
	return NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries))
}
