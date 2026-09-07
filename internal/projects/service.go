package projects

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	db "github.com/portd/internal/db/generated"
)

var (
	ErrInvalid  = errors.New("invalid project data")
	ErrConflict = errors.New("project already exists")
	ErrNotFound = errors.New("project not found")
)

var (
	slugPattern      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	lifecycleAllowed = map[string]struct{}{
		"draft": {}, "ready": {}, "running": {}, "stopped": {}, "failed": {}, "archived": {},
	}
)

type Service struct {
	db      *sql.DB
	queries *db.Queries
}

func NewService(database *sql.DB, queries *db.Queries) *Service {
	return &Service{db: database, queries: queries}
}

// ProjectWithInterns is a project row plus its assigned interns.
type ProjectWithInterns struct {
	Project db.Project
	Interns []db.Intern
}

type CreateInput struct {
	Name            string
	Slug            string
	Description     string
	InternIDs       []string
	LifecycleStatus string
	ShouldRun       bool
}

type UpdateInput struct {
	Name            string
	Description     string
	InternIDs       []string
	LifecycleStatus string
	ShouldRun       bool
}

func (s *Service) List(ctx context.Context, search, lifecycleStatus string, isLive int64) ([]ProjectWithInterns, error) {
	search = strings.TrimSpace(search)
	if lifecycleStatus != "" {
		if _, ok := lifecycleAllowed[lifecycleStatus]; !ok {
			return nil, ErrInvalid
		}
	}
	rows, err := s.queries.ListProjects(ctx, db.ListProjectsParams{
		Column1:         search,
		Column2:         sql.NullString{String: search, Valid: true},
		Column3:         sql.NullString{String: search, Valid: true},
		Column4:         sql.NullString{String: search, Valid: true},
		Column5:         lifecycleStatus,
		LifecycleStatus: lifecycleStatus,
		Column7:         isLive,
		IsLive:          isLive,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ProjectWithInterns, 0, len(rows))
	for _, project := range rows {
		interns, err := s.queries.ListProjectInterns(ctx, project.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ProjectWithInterns{Project: project, Interns: interns})
	}
	return out, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (ProjectWithInterns, error) {
	project, err := s.queries.GetProjectBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectWithInterns{}, ErrNotFound
	}
	if err != nil {
		return ProjectWithInterns{}, err
	}
	interns, err := s.queries.ListProjectInterns(ctx, project.ID)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	return ProjectWithInterns{Project: project, Interns: interns}, nil
}

func (s *Service) ListActiveInterns(ctx context.Context) ([]db.Intern, error) {
	return s.queries.ListActiveInterns(ctx)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (ProjectWithInterns, error) {
	name := strings.TrimSpace(in.Name)
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	lifecycle := in.LifecycleStatus
	if lifecycle == "" {
		lifecycle = "draft"
	}
	if err := validateProjectFields(name, slug, lifecycle, in.InternIDs); err != nil {
		return ProjectWithInterns{}, err
	}
	if err := s.ensureActiveInterns(ctx, in.InternIDs); err != nil {
		return ProjectWithInterns{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	shouldRun := int64(0)
	if in.ShouldRun {
		shouldRun = 1
	}
	id := uuid.NewString()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.queries.WithTx(tx)

	project, err := qtx.CreateProject(ctx, db.CreateProjectParams{
		ID:              id,
		Name:            name,
		Slug:            slug,
		Description:     strings.TrimSpace(in.Description),
		Directory:       "/var/portd/projects/" + slug,
		StartupCommand:  "./start.sh",
		ShouldRun:       shouldRun,
		IsLive:          0,
		LifecycleStatus: lifecycle,
		RouteSyncStatus: "pending",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if isConflict(err) {
		return ProjectWithInterns{}, ErrConflict
	}
	if err != nil {
		return ProjectWithInterns{}, err
	}
	if err := replaceInterns(ctx, qtx, project.ID, in.InternIDs, now); err != nil {
		return ProjectWithInterns{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectWithInterns{}, err
	}
	interns, err := s.queries.ListProjectInterns(ctx, project.ID)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	return ProjectWithInterns{Project: project, Interns: interns}, nil
}

func (s *Service) Update(ctx context.Context, slug string, in UpdateInput) (ProjectWithInterns, error) {
	existing, err := s.queries.GetProjectBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectWithInterns{}, ErrNotFound
	}
	if err != nil {
		return ProjectWithInterns{}, err
	}

	name := strings.TrimSpace(in.Name)
	lifecycle := in.LifecycleStatus
	if lifecycle == "" {
		lifecycle = existing.LifecycleStatus
	}
	if err := validateProjectFields(name, existing.Slug, lifecycle, in.InternIDs); err != nil {
		return ProjectWithInterns{}, err
	}
	if err := s.ensureActiveInterns(ctx, in.InternIDs); err != nil {
		return ProjectWithInterns{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	shouldRun := int64(0)
	if in.ShouldRun {
		shouldRun = 1
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.queries.WithTx(tx)

	project, err := qtx.UpdateProject(ctx, db.UpdateProjectParams{
		Name:            name,
		Description:     strings.TrimSpace(in.Description),
		ShouldRun:       shouldRun,
		LifecycleStatus: lifecycle,
		UpdatedAt:       now,
		ID:              existing.ID,
	})
	if err != nil {
		return ProjectWithInterns{}, err
	}
	if err := replaceInterns(ctx, qtx, project.ID, in.InternIDs, now); err != nil {
		return ProjectWithInterns{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectWithInterns{}, err
	}
	interns, err := s.queries.ListProjectInterns(ctx, project.ID)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	return ProjectWithInterns{Project: project, Interns: interns}, nil
}

func validateProjectFields(name, slug, lifecycle string, internIDs []string) error {
	if len(name) < 2 || len(name) > 80 {
		return ErrInvalid
	}
	if !slugPattern.MatchString(slug) {
		return ErrInvalid
	}
	if _, ok := lifecycleAllowed[lifecycle]; !ok {
		return ErrInvalid
	}
	if len(internIDs) < 1 {
		return ErrInvalid
	}
	seen := make(map[string]struct{}, len(internIDs))
	for _, id := range internIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return ErrInvalid
		}
		if _, ok := seen[id]; ok {
			return ErrInvalid
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (s *Service) ensureActiveInterns(ctx context.Context, internIDs []string) error {
	for _, id := range internIDs {
		intern, err := s.queries.GetIntern(ctx, strings.TrimSpace(id))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalid
		}
		if err != nil {
			return err
		}
		if intern.Active != 1 {
			return ErrInvalid
		}
	}
	return nil
}

func replaceInterns(ctx context.Context, q *db.Queries, projectID string, internIDs []string, now string) error {
	if err := q.DeleteProjectInterns(ctx, projectID); err != nil {
		return err
	}
	for _, id := range internIDs {
		if err := q.InsertProjectIntern(ctx, db.InsertProjectInternParams{
			ProjectID: projectID,
			InternID:  strings.TrimSpace(id),
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func slugify(name string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func isConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
