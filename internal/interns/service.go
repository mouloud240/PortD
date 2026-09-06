package interns

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/portd/internal/db/generated"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalid   = errors.New("invalid intern data")
	ErrConflict  = errors.New("intern already exists")
	ErrNotFound  = errors.New("intern not found")
)

func isConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

type Service struct{ queries *db.Queries }

func NewService(queries *db.Queries) *Service { return &Service{queries: queries} }

func (s *Service) List(ctx context.Context, search string, active int64) ([]db.Intern, error) {
	search = strings.TrimSpace(search)
	return s.queries.ListInterns(ctx, db.ListInternsParams{Column1: search, Column2: sql.NullString{String: search, Valid: true}, Column3: sql.NullString{String: search, Valid: true}, Column4: sql.NullString{String: search, Valid: true}, Column5: active, Active: active})
}

func (s *Service) Get(ctx context.Context, id string) (db.Intern, error) {
	intern, err := s.queries.GetIntern(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Intern{}, ErrNotFound
	}
	return intern, err
}

func (s *Service) Create(ctx context.Context, fullName, email, identifier, password string) (db.Intern, error) {
	if strings.TrimSpace(fullName) == "" || strings.TrimSpace(identifier) == "" || password == "" || len(password) > 72 {
		return db.Intern{}, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return db.Intern{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	intern, err := s.queries.CreateIntern(ctx, db.CreateInternParams{ID: uuid.NewString(), FullName: strings.TrimSpace(fullName), Email: null(email), Identifier: null(identifier), PasswordHash: string(hash), Active: 1, CreatedAt: now, UpdatedAt: now})
	if isConflict(err) {
		return db.Intern{}, ErrConflict
	}
	return intern, err
}

func (s *Service) Update(ctx context.Context, id, fullName, email, identifier string, active bool) (db.Intern, error) {
	if strings.TrimSpace(fullName) == "" || strings.TrimSpace(identifier) == "" {
		return db.Intern{}, ErrInvalid
	}
	value := int64(0)
	if active {
		value = 1
	}
	intern, err := s.queries.UpdateIntern(ctx, db.UpdateInternParams{ID: id, FullName: strings.TrimSpace(fullName), Email: null(email), Identifier: null(identifier), Active: value, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if isConflict(err) {
		return db.Intern{}, ErrConflict
	}
	return intern, err
}

func (s *Service) ResetPassword(ctx context.Context, id, password string) error {
	if password == "" || len(password) > 72 {
		return ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.queries.UpdateInternPassword(ctx, db.UpdateInternPasswordParams{ID: id, PasswordHash: string(hash), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Service) Deactivate(ctx context.Context, id string) error {
	return s.queries.DeactivateIntern(ctx, db.DeactivateInternParams{ID: id, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Service) Delete(ctx context.Context, id string) error {
	count, err := s.queries.CountInternAssignments(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("intern has assigned projects")
	}
	return s.queries.DeleteIntern(ctx, id)
}

func null(value string) sql.NullString {
	return sql.NullString{String: strings.TrimSpace(value), Valid: strings.TrimSpace(value) != ""}
}
