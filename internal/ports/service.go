package ports

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	db "github.com/portd/internal/db/generated"
)

// Service reconciles live ports into port_observations on each query.
// Observation ProjectID NULL = unregistered (no row in ports table).
type Service struct {
	queries *db.Queries
	scanner Scanner
}

// NewService wires generated queries to any Scanner (gopsutil or fake).
func NewService(queries *db.Queries, scanner Scanner) *Service {
	return &Service{queries: queries, scanner: scanner}
}

// Snapshot scans, upserts current ports, prunes stale rows, returns the page rows.
func (s *Service) Snapshot(ctx context.Context) ([]db.PortObservation, error) {
	live, err := s.scanner.ListeningPorts(ctx)
	if err != nil {
		return nil, err
	}
	live = FilterRange(live) // ponytail: fakes stay unfiltered, prod already is
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, p := range live {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var projectID sql.NullString
		if id, err := s.queries.GetPortProjectID(ctx, int64(p.Port)); err == nil {
			projectID = sql.NullString{String: id, Valid: true}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		var pid sql.NullInt64
		if p.PID > 0 {
			pid = sql.NullInt64{Int64: int64(p.PID), Valid: true}
		}
		var name sql.NullString
		if p.ProcessName != "" {
			name = sql.NullString{String: p.ProcessName, Valid: true}
		}
		if _, err := s.queries.UpsertPortObservation(ctx, db.UpsertPortObservationParams{
			Column1:     strconv.Itoa(p.Port),
			Port:        int64(p.Port),
			ProcessName: name,
			ProcessID:   pid,
			ProjectID:   projectID,
			ObservedAt:  now,
		}); err != nil {
			return nil, err
		}
	}
	// Rows older than this snapshot are no longer listening.
	if err := s.queries.DeleteStalePortObservations(ctx, now); err != nil {
		return nil, err
	}
	return s.queries.ListPortObservations(ctx)
}
