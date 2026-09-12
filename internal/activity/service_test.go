package activity

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/db/generated"
	_ "modernc.org/sqlite"
)

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
	for _, name := range []string{"000001_initial_schema.up.sql", "000002_add_routes.up.sql", "000003_add_local_auth.up.sql"} {
		schema, err := os.ReadFile(filepath.Join("..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(string(schema)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	queries := db.New(database)
	service := NewService(queries)
	service.Start()
	t.Cleanup(service.Close)
	return service, queries
}

func waitFor(t *testing.T, want int, list func() int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := list(); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d rows, have %d", want, list())
}

func TestRecordAndList(t *testing.T) {
	t.Parallel()

	service, queries := testService(t)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := queries.CreateIntern(context.Background(), db.CreateInternParams{
		ID: "intern-1", FullName: "Test Intern", Active: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{Role: "intern", InternID: "intern-1"})
	service.Record(ctx, Event{EventType: ProjectCreate, EntityType: "project", EntityID: "demo", Detail: "Demo"})
	service.Record(context.Background(), Event{EventType: AuthLogin, EntityType: "session", Outcome: OutcomeFailure, Detail: "admin"})

	count := func() int {
		rows, err := service.List(context.Background(), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	waitFor(t, 2, count)

	rows, err := service.List(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].EventType != AuthLogin {
		t.Fatalf("newest = %q, want %q", rows[0].EventType, AuthLogin)
	}
	if rows[0].ActorInternID.Valid {
		t.Fatalf("failed login actor = %q, want NULL", rows[0].ActorInternID.String)
	}
	var internRow bool
	for _, row := range rows {
		if row.ActorInternID.Valid && row.ActorInternID.String == "intern-1" && row.EntityID.String == "demo" {
			internRow = true
		}
	}
	if !internRow {
		t.Fatalf("no row with intern actor and entity demo: %+v", rows)
	}

	filtered, err := service.List(context.Background(), Filter{EventType: ProjectCreate, Outcome: OutcomeSuccess})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].EventType != ProjectCreate {
		t.Fatalf("filtered = %+v, want one project.create", filtered)
	}

	searched, err := service.List(context.Background(), Filter{Search: "dem"})
	if err != nil {
		t.Fatal(err)
	}
	if len(searched) != 1 {
		t.Fatalf("searched = %d rows, want 1", len(searched))
	}

	byCategory, err := service.List(context.Background(), Filter{Category: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byCategory) != 1 || byCategory[0].EventType != ProjectCreate {
		t.Fatalf("byCategory = %+v, want one project.create", byCategory)
	}
}

func TestCloseDrainsQueuedEvents(t *testing.T) {
	t.Parallel()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	schema, err := os.ReadFile(filepath.Join("..", "db", "migrations", "000001_initial_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	service := NewService(db.New(database))
	service.Start()
	for i := 0; i < 10; i++ {
		service.Record(context.Background(), Event{EventType: ProjectUpdate, EntityType: "project", EntityID: "demo"})
	}
	service.Close()

	rows, err := service.List(context.Background(), Filter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("rows = %d, want 10 after drain", len(rows))
	}
}

func TestRecordDropsWhenFull(t *testing.T) {
	t.Parallel()

	// Use an unstarted service so the queue never drains.
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	unstarted := NewService(db.New(database))
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < queueSize+10; i++ {
			unstarted.Record(context.Background(), Event{EventType: ProjectUpdate})
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Record blocked on a full queue")
	}
	if got := len(unstarted.events); got != queueSize {
		t.Fatalf("queued = %d, want cap %d", got, queueSize)
	}
}

func TestRecordFailureSwallowed(t *testing.T) {
	t.Parallel()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// No schema: every insert fails. The worker must log and continue.
	service := NewService(db.New(database))
	service.Start()
	defer service.Close()
	_ = database.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		service.Record(context.Background(), Event{EventType: RuntimeStart, EntityType: "project", EntityID: "demo"})
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Record blocked while worker errors")
	}
}
