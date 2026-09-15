package projects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/portd/internal/auth"
	db "github.com/portd/internal/db/generated"
	portsvc "github.com/portd/internal/ports"
	"github.com/portd/internal/runtime"
)

var (
	ErrInvalid  = errors.New("invalid project data")
	ErrConflict = errors.New("project already exists")
	ErrNotFound = errors.New("project not found")
	ErrScaffold = errors.New("could not prepare project directory")
)

const (
	AccessModeDirect  = "direct"
	AccessModeProxied = "proxied"
)

var (
	slugPattern      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	lifecycleAllowed = map[string]struct{}{
		"draft": {}, "ready": {}, "running": {}, "stopped": {}, "failed": {}, "archived": {},
	}
)

// defaultProjectsDir matches the historical hard-coded project location.
const defaultProjectsDir = "/var/portd/projects"

type Service struct {
	db          *sql.DB
	queries     *db.Queries
	baseURL     string
	projectsDir string
	scaffold    runtime.Scaffolder
}

func NewService(database *sql.DB, queries *db.Queries, baseURL, projectsDir string, scaffold runtime.Scaffolder) *Service {
	projectsDir = strings.TrimRight(strings.TrimSpace(projectsDir), "/")
	if projectsDir == "" {
		projectsDir = defaultProjectsDir
	}
	return &Service{db: database, queries: queries, baseURL: strings.TrimSpace(baseURL), projectsDir: projectsDir, scaffold: scaffold}
}

// ProjectURL builds the public project URL from the configured base URL and slug.
func (s *Service) ProjectURL(slug string) string {
	base := strings.TrimRight(s.baseURL, "/")
	slug = strings.Trim(slug, "/")
	if base == "" {
		return "/" + slug
	}
	return base + "/" + slug
}

// DirectProjectURL returns the project's URL on its assigned main port.
func (s *Service) DirectProjectURL(port int64) (string, error) {
	base, err := url.Parse(strings.TrimSpace(s.baseURL))
	if err != nil || base.Scheme == "" || base.Hostname() == "" {
		return "", fmt.Errorf("invalid base URL")
	}
	base.Path = ""
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	base.Host = net.JoinHostPort(base.Hostname(), strconv.FormatInt(port, 10))
	return strings.TrimRight(base.String(), "/") + "/", nil
}

func validAccessMode(mode string) bool {
	return mode == AccessModeDirect || mode == AccessModeProxied
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
	PortCount       int
	CreatorInternID string
	AccessMode      string
}

type UpdateInput struct {
	Name            string
	Description     string
	InternIDs       []string
	LifecycleStatus string
	ShouldRun       bool
	AccessMode      string
}

func (s *Service) SetStartupCommand(ctx context.Context, slug, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return ErrInvalid
	}
	if err := s.queries.SetProjectStartupCommand(ctx, db.SetProjectStartupCommandParams{
		StartupCommand: command,
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		Slug:           slug,
	}); err != nil {
		slog.Error("runtime configure failed", "slug", slug, "error", err)
		return err
	}
	slog.Info("runtime configured", "slug", slug, "command", command)
	return nil
}

func (s *Service) SetRuntimeIntent(ctx context.Context, slug string, shouldRun bool, lifecycle string) error {
	value := int64(0)
	if shouldRun {
		value = 1
	}
	if err := s.queries.SetProjectRuntimeIntent(ctx, db.SetProjectRuntimeIntentParams{
		ShouldRun:       value,
		LifecycleStatus: lifecycle,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		Slug:            slug,
	}); err != nil {
		slog.Error("runtime intent update failed", "slug", slug, "should_run", shouldRun, "error", err)
		return err
	}
	slog.Info("runtime intent updated", "slug", slug, "should_run", shouldRun, "lifecycle", lifecycle)
	return nil
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
	return s.withInterns(ctx, rows)
}

func (s *Service) ListForIntern(ctx context.Context, internID, search, lifecycleStatus string, isLive int64) ([]ProjectWithInterns, error) {
	search = strings.TrimSpace(search)
	if lifecycleStatus != "" {
		if _, ok := lifecycleAllowed[lifecycleStatus]; !ok {
			return nil, ErrInvalid
		}
	}
	rows, err := s.queries.ListInternProjects(ctx, db.ListInternProjectsParams{
		InternID:        internID,
		Column2:         search,
		Column3:         sql.NullString{String: search, Valid: true},
		Column4:         sql.NullString{String: search, Valid: true},
		Column5:         sql.NullString{String: search, Valid: true},
		Column6:         lifecycleStatus,
		LifecycleStatus: lifecycleStatus,
		Column8:         isLive,
		IsLive:          isLive,
	})
	if err != nil {
		return nil, err
	}
	return s.withInterns(ctx, rows)
}

func (s *Service) withInterns(ctx context.Context, rows []db.Project) ([]ProjectWithInterns, error) {
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

func (s *Service) CanManage(ctx context.Context, slug string, principal auth.Principal) (bool, error) {
	if principal.IsAdmin() {
		return true, nil
	}
	if principal.InternID == "" {
		return false, nil
	}
	project, err := s.queries.GetProjectBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	count, err := s.queries.IsProjectMember(ctx, db.IsProjectMemberParams{ProjectID: project.ID, InternID: principal.InternID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
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

type Overview struct {
	Active    int
	Live      int
	Recent    []ProjectWithInterns
	Lifecycle []LifecycleCount
}

// LifecycleCount is one bar of the overview lifecycle breakdown.
type LifecycleCount struct {
	Status  string
	Label   string
	Class   string
	Color   string
	Count   int
	Percent int
}

// lifecycleOrder fixes the dashboard bar order; archived is excluded
// because Overview only counts active projects.
var lifecycleOrder = []LifecycleCount{
	{Status: "draft", Label: "Brouillon", Class: "draft", Color: "#2d5fb3"},
	{Status: "ready", Label: "Prêt", Class: "ready", Color: "#9a5500"},
	{Status: "running", Label: "En cours", Class: "running", Color: "#087f44"},
	{Status: "stopped", Label: "Arrêté", Class: "stopped", Color: "#bd2d2d"},
	{Status: "failed", Label: "Échoué", Class: "failed", Color: "#bd2d2d"},
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	rows, err := s.queries.ListProjects(ctx, db.ListProjectsParams{
		Column1:         "",
		Column2:         sql.NullString{},
		Column3:         sql.NullString{},
		Column4:         sql.NullString{},
		Column5:         "",
		LifecycleStatus: "",
		Column7:         -1,
		IsLive:          0,
	})
	if err != nil {
		return Overview{}, err
	}
	out := Overview{}
	counts := make(map[string]int, len(lifecycleOrder))
	for _, project := range rows {
		if project.LifecycleStatus == "archived" {
			continue
		}
		out.Active++
		counts[project.LifecycleStatus]++
		if project.IsLive == 1 {
			out.Live++
		}
		if len(out.Recent) >= 4 {
			continue
		}
		interns, err := s.queries.ListProjectInterns(ctx, project.ID)
		if err != nil {
			return Overview{}, err
		}
		out.Recent = append(out.Recent, ProjectWithInterns{Project: project, Interns: interns})
	}
	// ponytail: tallied in memory from rows already fetched above; switch to
	// SELECT lifecycle_status, COUNT(*) ... GROUP BY when projects number in the thousands.
	peak := 0
	for _, entry := range lifecycleOrder {
		if counts[entry.Status] > peak {
			peak = counts[entry.Status]
		}
	}
	for _, entry := range lifecycleOrder {
		entry.Count = counts[entry.Status]
		if peak > 0 {
			entry.Percent = entry.Count * 100 / peak
		}
		out.Lifecycle = append(out.Lifecycle, entry)
	}
	return out, nil
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
	accessMode := in.AccessMode
	if accessMode == "" {
		accessMode = AccessModeDirect
	}
	if !validAccessMode(accessMode) {
		return ProjectWithInterns{}, ErrInvalid
	}
	internIDs := in.InternIDs
	if in.CreatorInternID != "" {
		creatorIncluded := false
		for _, id := range internIDs {
			if strings.TrimSpace(id) == in.CreatorInternID {
				creatorIncluded = true
				break
			}
		}
		if !creatorIncluded {
			internIDs = append(internIDs, in.CreatorInternID)
		}
	}
	if err := validateProjectFields(name, slug, lifecycle, internIDs); err != nil {
		return ProjectWithInterns{}, err
	}
	if in.PortCount < 1 || in.PortCount > 5 {
		return ProjectWithInterns{}, ErrInvalid
	}
	if err := s.ensureActiveInterns(ctx, internIDs); err != nil {
		return ProjectWithInterns{}, err
	}

	// Scaffold the directory before the row exists so a disk failure
	// aborts creation instead of leaving a row without a home.
	directory := filepath.Join(s.projectsDir, slug)
	if s.scaffold != nil {
		if err := s.scaffold.Create(ctx, runtime.Scaffold{
			Directory:      directory,
			Slug:           slug,
			StartupCommand: "./start.sh",
		}); err != nil {
			return ProjectWithInterns{}, fmt.Errorf("%w: %v", ErrScaffold, err)
		}
		slog.Info("project directory scaffolded", "slug", slug, "directory", directory)
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
		Directory:       directory,
		StartupCommand:  "./start.sh",
		ShouldRun:       shouldRun,
		IsLive:          0,
		LifecycleStatus: lifecycle,
		RouteSyncStatus: "pending",
		AccessMode:      accessMode,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if isConflict(err) {
		return ProjectWithInterns{}, ErrConflict
	}
	if err != nil {
		return ProjectWithInterns{}, err
	}
	if err := replaceInterns(ctx, qtx, project.ID, internIDs, now); err != nil {
		return ProjectWithInterns{}, err
	}
	if _, err := portsvc.Allocate(ctx, qtx, project.ID, in.PortCount); err != nil {
		return ProjectWithInterns{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectWithInterns{}, err
	}
	interns, err := s.queries.ListProjectInterns(ctx, project.ID)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	if s.scaffold != nil {
		s.writeReadme(ctx, project, accessMode)
	}
	slog.Info("project created", "slug", slug, "dir", directory, "interns", len(internIDs), "ports", in.PortCount, "access", accessMode)
	return ProjectWithInterns{Project: project, Interns: interns}, nil
}

// writeReadme regenerates the project README with the committed details.
// A failure is logged but never fails creation: the directory and row
// already exist and the README can be rewritten later.
func (s *Service) writeReadme(ctx context.Context, project db.Project, accessMode string) {
	assigned, err := s.queries.ListProjectPorts(ctx, project.ID)
	if err != nil {
		slog.Error("project readme ports failed", "slug", project.Slug, "error", err)
		return
	}
	main, err := s.queries.GetProjectMainPort(ctx, project.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("project readme main port failed", "slug", project.Slug, "error", err)
		return
	}
	ports := make([]runtime.PortRef, 0, len(assigned))
	for _, port := range assigned {
		ports = append(ports, runtime.PortRef{Port: port.Port, Role: port.Role, Main: port.Port == main.Port})
	}
	directURL, err := s.DirectProjectURL(main.Port)
	if err != nil {
		slog.Error("project readme direct URL failed", "slug", project.Slug, "error", err)
		directURL = ""
	}
	if err := s.scaffold.WriteReadme(ctx, runtime.Scaffold{
		Directory:      project.Directory,
		Slug:           project.Slug,
		Name:           project.Name,
		Description:    project.Description,
		StartupCommand: project.StartupCommand,
		AccessMode:     accessMode,
		Ports:          ports,
		MainPort:       main.Port,
		PublicURL:      s.ProjectURL(project.Slug),
		DirectURL:      directURL,
	}); err != nil {
		slog.Error("project readme failed", "slug", project.Slug, "error", err)
		return
	}
	slog.Info("project readme written", "slug", project.Slug, "directory", project.Directory)
}

// DetectedCandidate is one disk folder plus whether it is already registered.
type DetectedCandidate struct {
	Name      string
	Slug      string
	Directory string
	Exists    bool
}

// Scan lists usable disk folders without registering anything. Folders that
// are hidden, not directories, or have an unusable name are skipped.
func (s *Service) Scan(ctx context.Context) ([]DetectedCandidate, error) {
	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrScaffold, err)
	}
	known, err := s.List(ctx, "", "", -1)
	if err != nil {
		return nil, err
	}
	slugs := make(map[string]struct{}, len(known))
	dirs := make(map[string]struct{}, len(known))
	for _, item := range known {
		slugs[item.Project.Slug] = struct{}{}
		dirs[item.Project.Directory] = struct{}{}
	}
	var out []DetectedCandidate
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		slug := slugify(entry.Name())
		if !slugPattern.MatchString(slug) {
			continue
		}
		directory := filepath.Join(s.projectsDir, entry.Name())
		_, bySlug := slugs[slug]
		_, byDir := dirs[directory]
		out = append(out, DetectedCandidate{
			Name:      entry.Name(),
			Slug:      slug,
			Directory: directory,
			Exists:    bySlug || byDir,
		})
	}
	return out, nil
}

// Import registers the selected slugs from Scan and returns the added ones.
// Already-registered or unknown slugs are skipped. Detected projects get no
// interns and no ports; assign them from the project page afterwards.
func (s *Service) Import(ctx context.Context, slugs []string) ([]string, error) {
	wanted := make(map[string]struct{}, len(slugs))
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug != "" {
			wanted[slug] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	candidates, err := s.Scan(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var added []string
	for _, c := range candidates {
		if _, ok := wanted[c.Slug]; !ok || c.Exists {
			continue
		}
		_, err := s.queries.CreateProject(ctx, db.CreateProjectParams{
			ID:              uuid.NewString(),
			Name:            c.Name,
			Slug:            c.Slug,
			Description:     "",
			Directory:       c.Directory,
			StartupCommand:  "./start.sh",
			ShouldRun:       0,
			IsLive:          0,
			LifecycleStatus: "draft",
			RouteSyncStatus: "pending",
			AccessMode:      AccessModeDirect,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
		if isConflict(err) {
			continue
		}
		if err != nil {
			return added, err
		}
		added = append(added, c.Slug)
		slog.Info("project detected", "slug", c.Slug, "directory", c.Directory)
	}
	return added, nil
}

// Detect registers all unregistered disk folders (pre-selection behavior).
func (s *Service) Detect(ctx context.Context) ([]string, error) {
	candidates, err := s.Scan(ctx)
	if err != nil {
		return nil, err
	}
	var fresh []string
	for _, c := range candidates {
		if !c.Exists {
			fresh = append(fresh, c.Slug)
		}
	}
	return s.Import(ctx, fresh)
}

// MissingStartup reports whether a project directory holds neither
// start.sh nor start.bat.
func MissingStartup(directory string) bool {
	if directory == "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(directory, "start.sh")); err == nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(directory, "start.bat")); err == nil {
		return false
	}
	return true
}

func (s *Service) Update(ctx context.Context, slug string, in UpdateInput) (ProjectWithInterns, error) {
	existing, err := s.queries.GetProjectBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectWithInterns{}, ErrNotFound
	}
	if err != nil {
		return ProjectWithInterns{}, err
	}
	accessMode := in.AccessMode
	if accessMode == "" {
		accessMode = existing.AccessMode
	}
	if !validAccessMode(accessMode) {
		return ProjectWithInterns{}, ErrInvalid
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
		AccessMode:      accessMode,
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
	slog.Info("project updated", "slug", slug, "lifecycle", lifecycle, "access", accessMode)
	return ProjectWithInterns{Project: project, Interns: interns}, nil
}

func (s *Service) Archive(ctx context.Context, slug string) (ProjectWithInterns, error) {
	existing, err := s.queries.GetProjectBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectWithInterns{}, ErrNotFound
	}
	if err != nil {
		return ProjectWithInterns{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	project, err := s.queries.UpdateProject(ctx, db.UpdateProjectParams{
		Name:            existing.Name,
		Description:     existing.Description,
		ShouldRun:       0,
		LifecycleStatus: "archived",
		AccessMode:      existing.AccessMode,
		UpdatedAt:       now,
		ID:              existing.ID,
	})
	if err != nil {
		return ProjectWithInterns{}, err
	}
	interns, err := s.queries.ListProjectInterns(ctx, project.ID)
	if err != nil {
		return ProjectWithInterns{}, err
	}
	slog.Info("project archived", "slug", slug)
	return ProjectWithInterns{Project: project, Interns: interns}, nil
}

func (s *Service) projectIDBySlug(ctx context.Context, slug string) (string, error) {
	project, err := s.queries.GetProjectBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return project.ID, nil
}

func (s *Service) AddPorts(ctx context.Context, slug string, n int) error {
	if n < 1 || n > 5 {
		return ErrInvalid
	}
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ports, err := portsvc.Allocate(ctx, s.queries.WithTx(tx), projectID, n)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	slog.Info("ports allocated", "slug", slug, "count", n, "ports", portNumbers(ports))
	return nil
}

func (s *Service) ClaimPort(ctx context.Context, slug string, port int) error {
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if _, err = portsvc.ClaimPort(ctx, s.queries, projectID, port); err != nil {
		slog.Error("port claim failed", "slug", slug, "port", port, "error", err)
		return err
	}
	slog.Info("port claimed", "slug", slug, "port", port)
	return nil
}

func (s *Service) ReleasePort(ctx context.Context, slug string, port int64) error {
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if err := portsvc.ReleasePort(ctx, s.queries, projectID, port); err != nil {
		slog.Error("port release failed", "slug", slug, "port", port, "error", err)
		return err
	}
	slog.Info("port released", "slug", slug, "port", port)
	return nil
}

func (s *Service) ReplaceMainPort(ctx context.Context, slug string, claim int, releaseOld bool) (int64, bool, error) {
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return 0, false, err
	}
	newPort, released, err := portsvc.ReplaceMain(ctx, s.db, s.queries, projectID, claim, releaseOld)
	if err != nil {
		slog.Error("port replace-main failed", "slug", slug, "claim", claim, "error", err)
		return 0, false, err
	}
	slog.Info("port replace-main", "slug", slug, "new_port", newPort, "old_released", released)
	return newPort, released, nil
}

func (s *Service) PromotePort(ctx context.Context, slug string, port int64) error {
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if err := portsvc.PromoteMain(ctx, s.db, s.queries, projectID, port); err != nil {
		slog.Error("port promote failed", "slug", slug, "port", port, "error", err)
		return err
	}
	slog.Info("port promoted to main", "slug", slug, "port", port)
	return nil
}

func (s *Service) AddHealthcheck(ctx context.Context, slug, endpoint string, expectedStatus int) (db.ProjectHealthcheck, error) {
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return db.ProjectHealthcheck{}, err
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || len(endpoint) > 2048 || !validHealthEndpoint(endpoint) {
		return db.ProjectHealthcheck{}, ErrInvalid
	}
	if expectedStatus == 0 {
		expectedStatus = 200
	}
	if expectedStatus < 200 || expectedStatus > 599 {
		return db.ProjectHealthcheck{}, ErrInvalid
	}
	check, err := s.queries.AddProjectHealthcheck(ctx, db.AddProjectHealthcheckParams{
		ID:             uuid.NewString(),
		ProjectID:      projectID,
		Endpoint:       endpoint,
		ExpectedStatus: int64(expectedStatus),
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		slog.Error("healthcheck add failed", "slug", slug, "endpoint", endpoint, "error", err)
		return db.ProjectHealthcheck{}, err
	}
	slog.Info("healthcheck added", "slug", slug, "id", check.ID, "endpoint", endpoint, "expect", expectedStatus)
	return check, nil
}

func (s *Service) RemoveHealthcheck(ctx context.Context, slug, id string) error {
	projectID, err := s.projectIDBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if err := s.queries.DeleteProjectHealthcheck(ctx, db.DeleteProjectHealthcheckParams{ID: id, ProjectID: projectID}); err != nil {
		slog.Error("healthcheck remove failed", "slug", slug, "id", id, "error", err)
		return err
	}
	slog.Info("healthcheck removed", "slug", slug, "id", id)
	return nil
}

func validHealthEndpoint(endpoint string) bool {
	parsed, err := url.ParseRequestURI(endpoint)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
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

// ListPorts returns ports assigned to a project.
func (s *Service) ListPorts(ctx context.Context, projectID string) ([]db.Port, error) {
	return s.queries.ListProjectPorts(ctx, projectID)
}

// ListPortObservations returns the latest observed listening ports.
func (s *Service) ListPortObservations(ctx context.Context) ([]db.PortObservation, error) {
	return s.queries.ListPortObservations(ctx)
}

// ListHealthchecks returns healthchecks configured for a project.
func (s *Service) ListHealthchecks(ctx context.Context, projectID string) ([]db.ProjectHealthcheck, error) {
	return s.queries.ListProjectHealthchecks(ctx, projectID)
}

// MainPort returns the project's main port.
func (s *Service) MainPort(ctx context.Context, projectID string) (db.Port, error) {
	return s.queries.GetProjectMainPort(ctx, projectID)
}

func portNumbers(ports []db.Port) []int64 {
	out := make([]int64, 0, len(ports))
	for _, p := range ports {
		out = append(out, p.Port)
	}
	return out
}
