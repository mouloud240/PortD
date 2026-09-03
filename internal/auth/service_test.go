package auth

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portd/internal/db/generated"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func TestLoginAndLogout(t *testing.T) {
	t.Parallel()
	service, queries := testService(t)
	createIntern(t, queries, "intern-1", "intern", "secret-password")

	session, err := service.Login(context.Background(), "intern", "secret-password")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if session.Principal.Role != "intern" || session.Principal.InternID != "intern-1" {
		t.Fatalf("principal = %+v", session.Principal)
	}
	if _, err := service.Session(context.Background(), session.Token); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if err := service.Logout(context.Background(), session.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Session(context.Background(), session.Token); err == nil {
		t.Fatal("session remains valid after logout")
	}
}

func TestLoginRejectsBadCredentialsAndExpiredSessions(t *testing.T) {
	t.Parallel()
	service, queries := testService(t)
	createIntern(t, queries, "intern-1", "intern", "secret-password")
	if _, err := service.Login(context.Background(), "intern", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("error = %v, want %v", err, ErrInvalidCredentials)
	}

	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.sessionTTL = time.Minute
	session, err := service.Login(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatalf("admin login: %v", err)
	}
	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := service.Session(context.Background(), session.Token); err != ErrInvalidCredentials {
		t.Fatalf("error = %v, want expired session", err)
	}
}

func testService(t *testing.T) (*Service, *db.Queries) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
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
	return NewService(queries, "admin", "admin-password"), queries
}

func createIntern(t *testing.T, queries *db.Queries, id, username, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = queries.CreateIntern(context.Background(), db.CreateInternParams{
		ID: id, FullName: "Test Intern", Identifier: sql.NullString{String: username, Valid: true},
		PasswordHash: string(hash), Active: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
}
