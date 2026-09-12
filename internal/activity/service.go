// Package activity records audit events asynchronously.
//
// Record never blocks the caller and never returns an error: events are
// queued to a bounded channel drained by a single background worker, and a
// full queue drops the newest event with a warning log. Insert failures are
// logged and skipped. Callers must Start the worker once and Close it on
// shutdown to flush queued events.
package activity

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/portd/internal/auth"
	"github.com/portd/internal/db/generated"
)

// Event types recorded in activity_logs.
const (
	AuthLogin        = "auth.login"
	AuthLogout       = "auth.logout"
	ProjectCreate    = "project.create"
	ProjectUpdate    = "project.update"
	ProjectArchive   = "project.archive"
	RuntimeStart     = "runtime.start"
	RuntimeStop      = "runtime.stop"
	RuntimeConfigure = "runtime.configure"
	PortAllocate     = "port.allocate"
	PortRelease      = "port.release"
	PortClaim        = "port.claim"
	PortPromote      = "port.promote"
	HealthAdd        = "healthcheck.add"
	HealthRemove     = "healthcheck.remove"
	InternCreate     = "intern.create"
	InternUpdate     = "intern.update"
	ProfileUpdate    = "intern.profile"
)

// Outcomes recorded in activity_logs.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// queueSize bounds recorder memory; a full queue drops new events.
const queueSize = 256

// Event describes one audit entry. Actor overrides the session principal
// when set (used for login, where no session exists yet); empty means
// derive from ctx, with admins recorded as NULL.
type Event struct {
	Actor      string
	EventType  string
	EntityType string
	EntityID   string
	Outcome    string
	Detail     string
}

// Filter scopes List and Recent queries; empty fields match everything.
// Category is an event prefix such as project, runtime, port, intern,
// auth, or healthcheck.
type Filter struct {
	EventType string
	Category  string
	Outcome   string
	Search    string
	Limit     int64
	Offset    int64
}

type Service struct {
	queries *db.Queries
	events  chan db.RecordActivityParams
	wait    sync.WaitGroup
	once    sync.Once
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries, events: make(chan db.RecordActivityParams, queueSize)}
}

// Start launches the background insert worker. Call once.
func (s *Service) Start() {
	s.wait.Add(1)
	go s.run()
}

func (s *Service) run() {
	defer s.wait.Done()
	for params := range s.events {
		if err := s.queries.RecordActivity(context.Background(), params); err != nil {
			slog.Error("activity record failed", "event", params.EventType, "error", err)
		}
	}
}

// Close drains queued events and stops the worker.
func (s *Service) Close() {
	s.once.Do(func() { close(s.events) })
	s.wait.Wait()
}

// Record queues an event without blocking. Safe on a nil receiver so
// test-constructed handlers without a recorder keep working.
func (s *Service) Record(ctx context.Context, event Event) {
	if s == nil {
		return
	}
	outcome := event.Outcome
	if outcome == "" {
		outcome = OutcomeSuccess
	}
	actor := sql.NullString{}
	if event.Actor != "" {
		actor = sql.NullString{String: event.Actor, Valid: true}
	} else if principal, ok := auth.PrincipalFrom(ctx); ok && !principal.IsAdmin() && principal.InternID != "" {
		actor = sql.NullString{String: principal.InternID, Valid: true}
	}
	entity := sql.NullString{}
	if event.EntityID != "" {
		entity = sql.NullString{String: event.EntityID, Valid: true}
	}
	params := db.RecordActivityParams{
		ID:            uuid.NewString(),
		ActorInternID: actor,
		EventType:     event.EventType,
		EntityType:    event.EntityType,
		EntityID:      entity,
		Outcome:       outcome,
		Detail:        event.Detail,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	select {
	case s.events <- params:
	default:
		slog.Warn("activity queue full, dropping event", "event", event.EventType)
	}
}

// List returns events newest-first with pagination. Limit defaults to 50
// and caps at 200.
func (s *Service) List(ctx context.Context, filter Filter) ([]db.ActivityLog, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	return s.queries.ListActivity(ctx, db.ListActivityParams{
		EventType:  filter.EventType,
		Category:   filter.Category,
		Outcome:    filter.Outcome,
		Search:     filter.Search,
		PageOffset: offset,
		PageLimit:  limit,
	})
}

// Recent returns the newest n events for the dashboard feed.
func (s *Service) Recent(ctx context.Context, n int64) ([]db.ActivityLog, error) {
	return s.List(ctx, Filter{Limit: n})
}
