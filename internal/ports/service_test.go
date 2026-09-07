package ports

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	db "github.com/portd/internal/db/generated"
	_ "modernc.org/sqlite"
)

type stubScanner struct{ ports []ListeningPort }

func (s stubScanner) ListeningPorts(ctx context.Context) ([]ListeningPort, error) {
	return s.ports, nil
}

func testDB(t *testing.T) *db.Queries {
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
	t.Cleanup(func() {})
	queries := db.New(database)
	// Seed one project + registered port 3000.
	if _, err := database.Exec(`INSERT INTO projects (id, name, slug, description, directory, startup_command, should_run, is_live, lifecycle_status, route_sync_status, created_at, updated_at) VALUES ('p1','P','p','','/tmp','./start.sh',0,0,'draft','pending','t','t')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO ports (port, project_id, role, created_at) VALUES (3000, 'p1', 'main', 't')`); err != nil {
		t.Fatal(err)
	}
	// Stash raw db for service via queries; service only needs queries.
	_ = database
	return queries
}

func TestSnapshotResolvesAndPrunes(t *testing.T) {
	queries := testDB(t)
	svc := NewService(queries, stubScanner{ports: []ListeningPort{{Port: 3000, PID: 7, ProcessName: "node"}, {Port: 4000}}})

	rows, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	byPort := map[int64]db.PortObservation{}
	for _, r := range rows {
		byPort[r.Port] = r
	}
	if !byPort[3000].ProjectID.Valid || byPort[3000].ProjectID.String != "p1" {
		t.Fatalf("port 3000 project = %+v, want p1", byPort[3000].ProjectID)
	}
	if byPort[4000].ProjectID.Valid {
		t.Fatalf("port 4000 project = %+v, want NULL (unregistered)", byPort[4000].ProjectID)
	}

	// Second snapshot without 3000 prunes it; re-upsert of 4000 keeps one row.
	svc2 := NewService(queries, stubScanner{ports: []ListeningPort{{Port: 4000, PID: 9, ProcessName: "py"}}})
	rows, err = svc2.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Port != 4000 {
		t.Fatalf("rows = %+v, want only 4000", rows)
	}
	if !rows[0].ProcessID.Valid || rows[0].ProcessID.Int64 != 9 {
		t.Fatalf("port 4000 pid = %+v, want 9", rows[0].ProcessID)
	}
}
