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
	portsvc "github.com/portd/internal/ports"
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

	NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries, "https://portd.example.test"), testPorts(queries)).ServeHTTP(response, request)

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

	NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries, "https://portd.example.test"), testPorts(queries)).ServeHTTP(response, request)

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

	NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries, "https://portd.example.test"), testPorts(queries)).ServeHTTP(response, request)

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
	handler := NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries, "https://portd.example.test"), testPorts(queries))
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
	for _, name := range []string{"000001_initial_schema.up.sql", "000002_add_routes.up.sql", "000003_add_local_auth.up.sql", "000004_add_healthchecks.up.sql", "000005_add_access_mode.up.sql"} {
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
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func testHandler(t *testing.T, database *sql.DB, authService *auth.Service) http.Handler {
	t.Helper()
	queries := db.New(database)
	return NewHandler(authService, internsvc.NewService(queries), projectsvc.NewService(database, queries, "https://portd.example.test"), testPorts(queries))
}

func TestInternSeesAndManagesOwnProjects(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	initDB(t, database)
	queries := db.New(database)
	authService := auth.NewService(queries, "admin", "admin-password")
	internService := internsvc.NewService(queries)
	projectService := projectsvc.NewService(database, queries, "https://portd.example.test")
	handler := NewHandler(authService, internService, projectService, testPorts(queries))
	ctx := context.Background()

	mkIntern := func(name, identifier string) {
		t.Helper()
		if _, err := internService.Create(ctx, name, identifier+"@example.com", identifier, "secret-123"); err != nil {
			t.Fatalf("create intern %s: %v", identifier, err)
		}
	}
	mkIntern("Amine M", "amine")
	mkIntern("Sara K", "sara")
	amine, err := queries.GetActiveInternByIdentifier(ctx, sql.NullString{String: "amine", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	sara, err := queries.GetActiveInternByIdentifier(ctx, sql.NullString{String: "sara", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	mkProject := func(name, slug, internID string) {
		t.Helper()
		if _, err := projectService.Create(ctx, projectsvc.CreateInput{Name: name, Slug: slug, InternIDs: []string{internID}, PortCount: 1}); err != nil {
			t.Fatalf("create project %s: %v", slug, err)
		}
	}
	mkProject("Amine App", "amine-app", amine.ID)
	mkProject("Sara App", "sara-app", sara.ID)

	session, err := authService.Login(ctx, "amine", "secret-123")
	if err != nil {
		t.Fatalf("intern login: %v", err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	list := get("/projects")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", list.Code, http.StatusOK)
	}
	if body := list.Body.String(); !strings.Contains(body, "amine-app") || strings.Contains(body, "sara-app") {
		t.Fatalf("intern list leaks or hides projects")
	}
	if detail := get("/projects/amine-app"); detail.Code != http.StatusOK {
		t.Fatalf("own detail status = %d, want %d", detail.Code, http.StatusOK)
	}
	if foreign := get("/projects/sara-app"); foreign.Code != http.StatusForbidden {
		t.Fatalf("foreign detail status = %d, want %d", foreign.Code, http.StatusForbidden)
	}

	form := url.Values{"name": {"Self Made"}, "description": {"intern created"}}
	request := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, request)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("intern create status = %d, want %d", created.Code, http.StatusSeeOther)
	}
	own, err := projectService.GetBySlug(ctx, "self-made")
	if err != nil {
		t.Fatalf("created project missing: %v", err)
	}
	member := false
	for _, intern := range own.Interns {
		if intern.ID == amine.ID {
			member = true
		}
	}
	if !member {
		t.Fatalf("creator was not assigned to own project")
	}
}

type stubPortScanner struct{}

func (stubPortScanner) ListeningPorts(ctx context.Context) ([]portsvc.ListeningPort, error) {
	return nil, nil
}

func testPorts(queries *db.Queries) *portsvc.Service {
	return portsvc.NewService(queries, stubPortScanner{})
}
