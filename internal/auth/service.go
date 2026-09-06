// Package auth implements local browser authentication.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/portd/internal/db/generated"
	"golang.org/x/crypto/bcrypt"
)

const SessionCookieName = "portd_session"

var ErrInvalidCredentials = errors.New("invalid credentials")

type Principal struct {
	Role      string
	InternID  string
	CSRFToken string
}

func (p Principal) IsAdmin() bool { return p.Role == "admin" }

// AdminUsername exposes the configured admin login for display purposes.
func (s *Service) AdminUsername() string { return s.adminUsername }

type Session struct {
	Token     string
	CSRFToken string
	Principal Principal
}

type Service struct {
	queries       *db.Queries
	adminUsername string
	adminPassword string
	now           func() time.Time
	sessionTTL    time.Duration
}

func NewService(queries *db.Queries, adminUsername, adminPassword string) *Service {
	return &Service{
		queries:       queries,
		adminUsername: adminUsername,
		adminPassword: adminPassword,
		now:           time.Now,
		sessionTTL:    24 * time.Hour,
	}
}

func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	if username == s.adminUsername && s.adminUsername != "" && subtle.ConstantTimeCompare([]byte(password), []byte(s.adminPassword)) == 1 {
		return s.createSession(ctx, Principal{Role: "admin"})
	}
	if len(password) > 72 {
		return Session{}, ErrInvalidCredentials
	}

	intern, err := s.queries.GetActiveInternByIdentifier(ctx, sql.NullString{String: username, Valid: true})
	if err != nil || bcrypt.CompareHashAndPassword([]byte(intern.PasswordHash), []byte(password)) != nil {
		return Session{}, ErrInvalidCredentials
	}
	return s.createSession(ctx, Principal{Role: "intern", InternID: intern.ID})
}

func (s *Service) Session(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrInvalidCredentials
	}
	session, err := s.queries.GetSessionByTokenHash(ctx, db.GetSessionByTokenHashParams{
		TokenHash: tokenHash(token),
		ExpiresAt: s.now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}
	principal := Principal{Role: session.Role, CSRFToken: session.CsrfToken}
	if session.InternID.Valid {
		principal.InternID = session.InternID.String
	}
	return principal, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.queries.DeleteSessionByTokenHash(ctx, tokenHash(token))
}

func (s *Service) createSession(ctx context.Context, principal Principal) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}
	csrfToken, err := randomToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate CSRF token: %w", err)
	}
	id, err := randomToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session ID: %w", err)
	}
	now := s.now().UTC()
	internID := sql.NullString{}
	if principal.InternID != "" {
		internID = sql.NullString{String: principal.InternID, Valid: true}
	}
	_, err = s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:        id,
		TokenHash: tokenHash(token),
		InternID:  internID,
		Role:      principal.Role,
		CsrfToken: csrfToken,
		ExpiresAt: now.Add(s.sessionTTL).Format(time.RFC3339Nano),
		CreatedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	principal.CSRFToken = csrfToken
	return Session{Token: token, CSRFToken: csrfToken, Principal: principal}, nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
