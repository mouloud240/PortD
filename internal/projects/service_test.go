package projects

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	db "github.com/portd/internal/db/generated"
	_ "modernc.org/sqlite"
)

func TestCreateListUpdateAssignsInterns(t *testing.T) {
	t.Parallel()
	service, queries := testService(t)
	createIntern(t, queries, "i1", "Alice")
	createIntern(t, queries, "i2", "Bob")

	created, err := service.Create(context.Background(), CreateInput{
		Name:      "Demo App",
		InternIDs: []string{"i1", "i2"},
		PortCount: 2,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Project.Slug != "demo-app" || created.Project.LifecycleStatus != "draft" || created.Project.AccessMode != AccessModeDirect {
		t.Fatalf("created = %+v", created.Project)
	}
	if len(created.Interns) != 2 {
		t.Fatalf("interns = %d, want 2", len(created.Interns))
	}
	ports, err := queries.ListProjectPorts(context.Background(), created.Project.ID)
	if err != nil {
		t.Fatalf("ports: %v", err)
	}
	if len(ports) != 2 || ports[0].Role != "main" || ports[1].Role != "internal" {
		t.Fatalf("ports = %+v, want main + internal", ports)
	}

	listed, err := service.List(context.Background(), "demo", "", -1)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %v len=%d", err, len(listed))
	}

	updated, err := service.Update(context.Background(), "demo-app", UpdateInput{
		Name:            "Demo App",
		Description:     "updated",
		InternIDs:       []string{"i1"},
		LifecycleStatus: "ready",
		ShouldRun:       true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Project.LifecycleStatus != "ready" || updated.Project.ShouldRun != 1 || updated.Project.Description != "updated" {
		t.Fatalf("updated = %+v", updated.Project)
	}
	if len(updated.Interns) != 1 || updated.Interns[0].ID != "i1" {
		t.Fatalf("interns = %+v", updated.Interns)
	}

	updated, err = service.Update(context.Background(), "demo-app", UpdateInput{
		Name:       "Demo App",
		InternIDs:  []string{"i1"},
		AccessMode: AccessModeProxied,
	})
	if err != nil || updated.Project.AccessMode != AccessModeProxied {
		t.Fatalf("access mode update = %+v, err=%v", updated.Project, err)
	}

	archived, err := service.Archive(context.Background(), "demo-app")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Project.LifecycleStatus != "archived" || archived.Project.ShouldRun != 0 {
		t.Fatalf("archived = %+v", archived.Project)
	}
}

func TestCreateRejectsInvalidAndConflict(t *testing.T) {
	t.Parallel()
	service, queries := testService(t)
	createIntern(t, queries, "i1", "Alice")

	if _, err := service.Create(context.Background(), CreateInput{Name: "X", InternIDs: []string{"i1"}, PortCount: 1}); err != ErrInvalid {
		t.Fatalf("short name error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.Create(context.Background(), CreateInput{Name: "Good Name", InternIDs: nil, PortCount: 1}); err != ErrInvalid {
		t.Fatalf("missing interns error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.Create(context.Background(), CreateInput{Name: "Good Name", InternIDs: []string{"missing"}, PortCount: 1}); err != ErrInvalid {
		t.Fatalf("unknown intern error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.Create(context.Background(), CreateInput{Name: "Good Name", Slug: "Bad_Slug", InternIDs: []string{"i1"}, PortCount: 1}); err != ErrInvalid {
		t.Fatalf("bad slug error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.Create(context.Background(), CreateInput{Name: "Good Name", Slug: "good-name", InternIDs: []string{"i1"}, PortCount: 6}); err != ErrInvalid {
		t.Fatalf("port count error = %v, want %v", err, ErrInvalid)
	}

	if _, err := service.Create(context.Background(), CreateInput{Name: "Good Name", Slug: "good-name", InternIDs: []string{"i1"}, PortCount: 1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := service.Create(context.Background(), CreateInput{Name: "Other", Slug: "good-name", InternIDs: []string{"i1"}, PortCount: 1}); err != ErrConflict {
		t.Fatalf("conflict error = %v, want %v", err, ErrConflict)
	}
}

func TestGetBySlugNotFound(t *testing.T) {
	t.Parallel()
	service, _ := testService(t)
	if _, err := service.GetBySlug(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("error = %v, want %v", err, ErrNotFound)
	}
}

func TestDirectProjectURL(t *testing.T) {
	svc := NewService(nil, nil, "http://10.243.1.20:8080/portd")
	for _, test := range []struct {
		port int64
		want string
	}{
		{7331, "http://10.243.1.20:7331/"},
	} {
		got, err := svc.DirectProjectURL(test.port)
		if err != nil {
			t.Fatalf("DirectProjectURL: %v", err)
		}
		if got != test.want {
			t.Fatalf("DirectProjectURL = %q, want %q", got, test.want)
		}
	}
}

func TestHealthcheckAddRemove(t *testing.T) {
	t.Parallel()
	service, queries := testService(t)
	createIntern(t, queries, "i1", "Alice")
	ctx := context.Background()
	created, err := service.Create(ctx, CreateInput{Name: "Health App", InternIDs: []string{"i1"}, PortCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddHealthcheck(ctx, created.Project.Slug, "notaurl", 200); err != ErrInvalid {
		t.Fatalf("bad endpoint error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.AddHealthcheck(ctx, created.Project.Slug, "http://example.test/health", 99); err != ErrInvalid {
		t.Fatalf("bad status error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.AddHealthcheck(ctx, created.Project.Slug, "/health", 200); err != ErrInvalid {
		t.Fatalf("relative endpoint error = %v, want %v", err, ErrInvalid)
	}
	check, err := service.AddHealthcheck(ctx, created.Project.Slug, "https://example.test/health", 0)
	if err != nil {
		t.Fatal(err)
	}
	if check.ExpectedStatus != 200 || check.Endpoint != "https://example.test/health" {
		t.Fatalf("healthcheck = %+v", check)
	}
	rows, err := queries.ListProjectHealthchecks(ctx, created.Project.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	if err := service.RemoveHealthcheck(ctx, created.Project.Slug, check.ID); err != nil {
		t.Fatal(err)
	}
	rows, err = queries.ListProjectHealthchecks(ctx, created.Project.ID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
}

func TestOverviewSkipsArchived(t *testing.T) {
	t.Parallel()
	service, queries := testService(t)
	createIntern(t, queries, "i1", "Alice")
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateInput{Name: "Live App", InternIDs: []string{"i1"}, PortCount: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, CreateInput{Name: "Old App", InternIDs: []string{"i1"}, PortCount: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Archive(ctx, "old-app"); err != nil {
		t.Fatal(err)
	}
	overview, err := service.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Active != 1 || len(overview.Recent) != 1 || overview.Recent[0].Project.Slug != "live-app" {
		t.Fatalf("overview = %+v", overview)
	}
	mainPort, err := service.MainPort(ctx, overview.Recent[0].Project.ID)
	if err != nil || mainPort.Port == 0 {
		t.Fatalf("main port = %+v, err = %v", mainPort, err)
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()
	if got := slugify(" Hello World! "); got != "hello-world" {
		t.Fatalf("slugify = %q", got)
	}
}

func TestProjectURL(t *testing.T) {
	t.Parallel()
	service, _ := testService(t)
	if got := service.ProjectURL("demo-app"); got != "https://portd.example.test/demo-app" {
		t.Fatalf("ProjectURL = %q", got)
	}
}

func testService(t *testing.T) (*Service, *db.Queries) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
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
	queries := db.New(database)
	return NewService(database, queries, "https://portd.example.test"), queries
}

func createIntern(t *testing.T, queries *db.Queries, id, name string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := queries.CreateIntern(context.Background(), db.CreateInternParams{
		ID:           id,
		FullName:     name,
		Email:        sql.NullString{String: id + "@example.com", Valid: true},
		Identifier:   sql.NullString{String: id, Valid: true},
		PasswordHash: "hash",
		Active:       1,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
}
