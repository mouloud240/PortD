package ports

import (
	"context"
	"errors"
	"strconv"
	"testing"

	db "github.com/portd/internal/db/generated"
)

func observe(t *testing.T, queries *db.Queries, port int) {
	t.Helper()
	_, err := queries.UpsertPortObservation(context.Background(), db.UpsertPortObservationParams{
		Column1:    strconv.Itoa(port),
		Port:       int64(port),
		ObservedAt: "t",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAllocateSkipsAssignedAndObserved(t *testing.T) {
	_, queries := testDB(t)
	ctx := context.Background()
	observe(t, queries, 3001)
	rows, err := Allocate(ctx, queries, "p1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Port != 3002 || rows[1].Port != 3003 {
		t.Fatalf("allocated = %+v, want [3002 3003]", rows)
	}
	for _, row := range rows {
		if row.Role != "internal" {
			t.Fatalf("port %d role = %q, want internal (p1 already has main)", row.Port, row.Role)
		}
	}
	if _, err := ClaimPort(ctx, queries, "p1", 3001); !errors.Is(err, ErrPortTaken) {
		t.Fatalf("claim taken error = %v, want %v", err, ErrPortTaken)
	}
	if _, err := ClaimPort(ctx, queries, "p1", 2999); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("claim range error = %v, want %v", err, ErrOutOfRange)
	}
	if _, err := Allocate(ctx, queries, "p1", MaxPort); !errors.Is(err, ErrNoPorts) {
		t.Fatalf("exhaust error = %v, want %v", err, ErrNoPorts)
	}
}

func TestPromoteAndReleaseGuards(t *testing.T) {
	database, queries := testDB(t)
	ctx := context.Background()
	if _, err := Allocate(ctx, queries, "p1", 1); err != nil {
		t.Fatal(err)
	}
	ports, err := queries.ListProjectPorts(ctx, "p1")
	if err != nil || len(ports) != 2 {
		t.Fatalf("ports = %+v, %v", ports, err)
	}
	internal := ports[1].Port
	if err := PromoteMain(ctx, database, queries, "p1", internal); err != nil {
		t.Fatal(err)
	}
	main, err := queries.GetProjectMainPort(ctx, "p1")
	if err != nil || main.Port != internal {
		t.Fatalf("main = %+v, %v, want %d", main, err, internal)
	}
	if err := ReleasePort(ctx, queries, "p1", internal); !errors.Is(err, ErrMainPort) {
		t.Fatalf("release main error = %v, want %v", err, ErrMainPort)
	}
	if err := ReleasePort(ctx, queries, "p1", 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("release missing error = %v, want %v", err, ErrNotFound)
	}
	observe(t, queries, 3000)
	if err := ReleasePort(ctx, queries, "p1", 3000); !errors.Is(err, ErrPortLive) {
		t.Fatalf("release live error = %v, want %v", err, ErrPortLive)
	}
}
