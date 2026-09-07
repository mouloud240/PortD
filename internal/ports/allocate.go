package ports

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	db "github.com/portd/internal/db/generated"
)

var (
	ErrNoPorts    = errors.New("not enough free ports")
	ErrPortTaken  = errors.New("port is already taken")
	ErrMainPort   = errors.New("promote another port to main first")
	ErrPortLive   = errors.New("port is in use")
	ErrNotFound   = errors.New("port not found")
	ErrOutOfRange = errors.New("port must be between 3000 and 9999")
)

func takenPorts(ctx context.Context, q *db.Queries) (map[int]bool, error) {
	var assigned []int64
	var observed []db.PortObservation
	var assignedErr, observedErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		assigned, assignedErr = q.ListAssignedPorts(ctx)
	}()
	go func() {
		defer wg.Done()
		observed, observedErr = q.ListPortObservations(ctx)
	}()
	wg.Wait()
	if assignedErr != nil {
		return nil, assignedErr
	}
	if observedErr != nil {
		return nil, observedErr
	}
	taken := make(map[int]bool, len(assigned)+len(observed))
	for _, port := range assigned {
		taken[int(port)] = true
	}
	for _, row := range observed {
		taken[int(row.Port)] = true
	}
	return taken, nil
}

func lowestFree(taken map[int]bool, n int) ([]int, error) {
	free := make([]int, 0, n)
	for port := MinPort; port <= MaxPort && len(free) < n; port++ {
		if !taken[port] {
			free = append(free, port)
		}
	}
	if len(free) < n {
		return nil, ErrNoPorts
	}
	return free, nil
}

// Allocate assigns n free ports to the project through q (caller's tx).
// The first inserted port is main when the project has none yet.
func Allocate(ctx context.Context, q *db.Queries, projectID string, n int) ([]db.Port, error) {
	taken, err := takenPorts(ctx, q)
	if err != nil {
		return nil, err
	}
	free, err := lowestFree(taken, n)
	if err != nil {
		return nil, err
	}
	existing, err := q.ListProjectPorts(ctx, projectID)
	if err != nil {
		return nil, err
	}
	mainExists := false
	for _, row := range existing {
		if row.Role == "main" {
			mainExists = true
			break
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	out := make([]db.Port, 0, len(free))
	for i, port := range free {
		role := "internal"
		if i == 0 && !mainExists {
			role = "main"
		}
		row, err := q.InsertPort(ctx, db.InsertPortParams{
			Port:      int64(port),
			ProjectID: projectID,
			Role:      role,
			CreatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// ReleasePort frees a port from the project. Main and live ports are refused.
func ReleasePort(ctx context.Context, q *db.Queries, projectID string, port int64) error {
	rows, err := q.ListProjectPorts(ctx, projectID)
	if err != nil {
		return err
	}
	role := ""
	for _, row := range rows {
		if row.Port == port {
			role = row.Role
		}
	}
	if role == "" {
		return ErrNotFound
	}
	if role == "main" {
		return ErrMainPort
	}
	observed, err := q.ListPortObservations(ctx)
	if err != nil {
		return err
	}
	for _, row := range observed {
		if row.Port == port {
			return ErrPortLive
		}
	}
	return q.DeletePort(ctx, db.DeletePortParams{Port: port, ProjectID: projectID})
}

// ClaimPort assigns one specific port when it is free by the union rule.
func ClaimPort(ctx context.Context, q *db.Queries, projectID string, port int) (db.Port, error) {
	if !InRange(port) {
		return db.Port{}, ErrOutOfRange
	}
	taken, err := takenPorts(ctx, q)
	if err != nil {
		return db.Port{}, err
	}
	if taken[port] {
		return db.Port{}, ErrPortTaken
	}
	existing, err := q.ListProjectPorts(ctx, projectID)
	if err != nil {
		return db.Port{}, err
	}
	role := "internal"
	if len(existing) == 0 {
		role = "main"
	}
	return q.InsertPort(ctx, db.InsertPortParams{
		Port:      int64(port),
		ProjectID: projectID,
		Role:      role,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// PromoteMain makes port the project's main port and demotes the old one.
func PromoteMain(ctx context.Context, database *sql.DB, q *db.Queries, projectID string, port int64) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := q.WithTx(tx)
	rows, err := qtx.ListProjectPorts(ctx, projectID)
	if err != nil {
		return err
	}
	found := false
	for _, row := range rows {
		if row.Port == port {
			found = true
			if row.Role == "main" {
				return tx.Commit()
			}
		}
	}
	if !found {
		return ErrNotFound
	}
	if current, err := qtx.GetProjectMainPort(ctx, projectID); err == nil {
		if _, err := qtx.SetPortRole(ctx, db.SetPortRoleParams{Role: "internal", Port: current.Port, ProjectID: projectID}); err != nil {
			return err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := qtx.SetPortRole(ctx, db.SetPortRoleParams{Role: "main", Port: port, ProjectID: projectID}); err != nil {
		return err
	}
	return tx.Commit()
}
