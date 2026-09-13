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

// Registration returns the project and interns associated with an observed port.
func (s *Service) Registration(ctx context.Context, projectID string) (db.Project, []db.Intern, error) {
	project, err := s.queries.GetProjectByID(ctx, projectID)
	if err != nil {
		return db.Project{}, nil, err
	}
	interns, err := s.queries.ListProjectInterns(ctx, projectID)
	if err != nil {
		return db.Project{}, nil, err
	}
	return project, interns, nil
}

// NewService wires generated queries to any Scanner (gopsutil or fake).
func NewService(queries *db.Queries, scanner Scanner) *Service {
	return &Service{queries: queries, scanner: scanner}
}

// Stats reports assigned ports and unregistered observations without scanning.
func (s *Service) Stats(ctx context.Context) (assigned, unknown int, err error) {
	ports, err := s.queries.ListAssignedPorts(ctx)
	if err != nil {
		return 0, 0, err
	}
	rows, err := s.queries.ListPortObservations(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, row := range rows {
		if !row.ProjectID.Valid {
			unknown++
		}
	}
	return len(ports), unknown, nil
}

// Reserved returns allocated ports with no live observation (e.g. a project
// holding ports while stopped). Call after Snapshot so observations are fresh.
func (s *Service) Reserved(ctx context.Context, observed []db.PortObservation) ([]db.Port, error) {
	live := make(map[int64]struct{}, len(observed))
	for _, row := range observed {
		live[row.Port] = struct{}{}
	}
	allocated, err := s.queries.ListAllPorts(ctx)
	if err != nil {
		return nil, err
	}
	// ponytail: diffed in memory; allocated ports number in the dozens.
	var out []db.Port
	for _, port := range allocated {
		if _, ok := live[port.Port]; !ok {
			out = append(out, port)
		}
	}
	return out, nil
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
