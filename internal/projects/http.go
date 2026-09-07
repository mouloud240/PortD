package projects

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
	portsvc "github.com/portd/internal/ports"
	"github.com/portd/views/pages"
)

func (s *Service) ListPage(w http.ResponseWriter, r *http.Request) error {
	search := r.URL.Query().Get("q")
	lifecycle := r.URL.Query().Get("lifecycle_status")
	liveParam := r.URL.Query().Get("is_live")
	isLive := int64(-1)
	switch liveParam {
	case "true":
		isLive = 1
	case "false":
		isLive = 0
	default:
		liveParam = ""
	}
	principal, _ := auth.PrincipalFrom(r.Context())
	if !principal.IsAdmin() && principal.InternID == "" {
		return httperr.Forbidden("Sign in as an intern or administrator.", nil)
	}
	items, err := s.scopedList(r, search, lifecycle, isLive)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			return httperr.BadRequest("Invalid project filter.", err)
		}
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectsPage(
		"Projects",
		"/projects",
		search,
		lifecycle,
		liveParam,
		s.ProjectListItems(r.Context(), items),
	))
}

func (s *Service) scopedList(r *http.Request, search, lifecycle string, isLive int64) ([]ProjectWithInterns, error) {
	principal, _ := auth.PrincipalFrom(r.Context())
	if principal.IsAdmin() {
		return s.List(r.Context(), search, lifecycle, isLive)
	}
	if principal.InternID == "" {
		return nil, ErrInvalid
	}
	return s.ListForIntern(r.Context(), principal.InternID, search, lifecycle, isLive)
}

func (s *Service) NewPage(w http.ResponseWriter, r *http.Request) error {
	selected := []string{}
	if principal, _ := auth.PrincipalFrom(r.Context()); !principal.IsAdmin() && principal.InternID != "" {
		selected = []string{principal.InternID}
	}
	options, err := s.internOptions(r, selected)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectFormPage(pages.ProjectFormData{
		Title:           "New project",
		Path:            "/projects",
		Action:          "/projects",
		LifecycleStatus: "draft",
		PortCount:       2,
		IsNew:           true,
		Interns:         options,
	}))
}

func (s *Service) CreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	shouldRun, _ := strconv.ParseBool(r.FormValue("should_run"))
	portCount := 2
	if raw := strings.TrimSpace(r.FormValue("ports_needed")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			portCount = n
		}
	}
	form := pages.ProjectFormData{
		Title:           "New project",
		Path:            "/projects",
		Action:          "/projects",
		Name:            r.FormValue("name"),
		Slug:            r.FormValue("slug"),
		Description:     r.FormValue("description"),
		LifecycleStatus: r.FormValue("lifecycle_status"),
		ShouldRun:       shouldRun,
		PortCount:       portCount,
		IsNew:           true,
	}
	internIDs := r.Form["intern_ids"]
	options, err := s.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	creator := ""
	if principal, _ := auth.PrincipalFrom(r.Context()); !principal.IsAdmin() {
		creator = principal.InternID
	}
	created, err := s.Create(r.Context(), CreateInput{
		Name:            form.Name,
		Slug:            form.Slug,
		Description:     form.Description,
		InternIDs:       internIDs,
		LifecycleStatus: form.LifecycleStatus,
		ShouldRun:       form.ShouldRun,
		PortCount:       portCount,
		CreatorInternID: creator,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			form.Error = "Name, at least one active intern, a valid lifecycle, and 1–5 ports are required."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProjectFormPage(form))
		case errors.Is(err, ErrConflict):
			form.Error = "A project with that slug already exists."
			return httperr.Render(w, r, http.StatusConflict, pages.ProjectFormPage(form))
		case errors.Is(err, portsvc.ErrNoPorts):
			form.Error = "Not enough free ports in range 3000–9999 for this project."
			return httperr.Render(w, r, http.StatusConflict, pages.ProjectFormPage(form))
		default:
			return err
		}
	}
	http.Redirect(w, r, "/projects/"+created.Project.Slug, http.StatusSeeOther)
	return nil
}

func (s *Service) DetailPage(w http.ResponseWriter, r *http.Request) error {
	detail, err := s.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httperr.NotFound("Project not found.", err)
		}
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectDetailPage(s.projectDetailData(r.Context(), detail, r.URL.Query().Get("port_error"))))
}

func (s *Service) EditPage(w http.ResponseWriter, r *http.Request) error {
	detail, err := s.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httperr.NotFound("Project not found.", err)
		}
		return err
	}
	selected := make([]string, 0, len(detail.Interns))
	for _, intern := range detail.Interns {
		selected = append(selected, intern.ID)
	}
	options, err := s.internOptions(r, selected)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectFormPage(pages.ProjectFormData{
		Title:           "Edit project",
		Path:            "/projects",
		Action:          "/projects/" + detail.Project.Slug,
		Name:            detail.Project.Name,
		Slug:            detail.Project.Slug,
		Description:     detail.Project.Description,
		LifecycleStatus: detail.Project.LifecycleStatus,
		ShouldRun:       detail.Project.ShouldRun == 1,
		IsLive:          detail.Project.IsLive == 1,
		Ports:           s.assignedPorts(r.Context(), detail.Project.ID),
		PortError:       r.URL.Query().Get("port_error"),
		Interns:         options,
	}))
}

func (s *Service) UpdatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	shouldRun, _ := strconv.ParseBool(r.FormValue("should_run"))
	form := pages.ProjectFormData{
		Title:           "Edit project",
		Path:            "/projects",
		Action:          "/projects/" + slug,
		Name:            r.FormValue("name"),
		Slug:            slug,
		Description:     r.FormValue("description"),
		LifecycleStatus: r.FormValue("lifecycle_status"),
		ShouldRun:       shouldRun,
	}
	internIDs := r.Form["intern_ids"]
	options, err := s.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	if projectID, err := s.projectIDBySlug(r.Context(), slug); err == nil {
		form.Ports = s.assignedPorts(r.Context(), projectID)
	}

	updated, err := s.Update(r.Context(), slug, UpdateInput{
		Name:            form.Name,
		Description:     form.Description,
		InternIDs:       internIDs,
		LifecycleStatus: form.LifecycleStatus,
		ShouldRun:       form.ShouldRun,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			form.Error = "Name, at least one active intern, and a valid lifecycle are required."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProjectFormPage(form))
		case errors.Is(err, ErrNotFound):
			return httperr.NotFound("Project not found.", err)
		default:
			return err
		}
	}
	form.IsLive = updated.Project.IsLive == 1
	http.Redirect(w, r, "/projects/"+updated.Project.Slug, http.StatusSeeOther)
	return nil
}

func (s *Service) ArchivePost(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	archived, err := s.Archive(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httperr.NotFound("Project not found.", err)
		}
		return err
	}
	http.Redirect(w, r, "/projects/"+archived.Project.Slug, http.StatusSeeOther)
	return nil
}

func (s *Service) PromotePortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	port, ok := parsePort(r.FormValue("port"))
	if !ok {
		return s.portBack(w, r, slug, portsvc.ErrOutOfRange)
	}
	if err := s.PromotePort(r.Context(), slug, port); err != nil {
		return s.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (s *Service) AllocatePortsPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	n := 1
	if raw := strings.TrimSpace(r.FormValue("count")); raw != "" {
		var err error
		n, err = strconv.Atoi(raw)
		if err != nil {
			return s.portBack(w, r, slug, ErrInvalid)
		}
	}
	if err := s.AddPorts(r.Context(), slug, n); err != nil {
		return s.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (s *Service) ReleasePortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	port, ok := parsePort(r.FormValue("port"))
	if !ok {
		return s.portBack(w, r, slug, portsvc.ErrOutOfRange)
	}
	if err := s.ReleasePort(r.Context(), slug, port); err != nil {
		return s.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (s *Service) ClaimPortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	if err != nil {
		return s.portBack(w, r, slug, ErrInvalid)
	}
	if err := s.ClaimPort(r.Context(), slug, port); err != nil {
		return s.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func parsePort(raw string) (int64, bool) {
	port, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

func portBackURL(r *http.Request, slug string) string {
	if next := r.FormValue("next"); strings.HasPrefix(next, "/projects/") {
		return next
	}
	return "/projects/" + slug
}

func (s *Service) portBack(w http.ResponseWriter, r *http.Request, slug string, err error) error {
	if errors.Is(err, ErrNotFound) || errors.Is(err, portsvc.ErrNotFound) {
		return httperr.NotFound("Project not found.", err)
	}
	msg := "Port action failed."
	switch {
	case errors.Is(err, ErrInvalid):
		msg = "Count must be between 1 and 5."
	case errors.Is(err, portsvc.ErrNoPorts):
		msg = "Not enough free ports in range 3000–9999."
	case errors.Is(err, portsvc.ErrPortTaken):
		msg = "That port is already taken."
	case errors.Is(err, portsvc.ErrOutOfRange):
		msg = "Port must be between 3000 and 9999."
	case errors.Is(err, portsvc.ErrMainPort):
		msg = "Promote another port to main before releasing this one."
	case errors.Is(err, portsvc.ErrPortLive):
		msg = "That port is in use. Stop the process first."
	default:
		return err
	}
	next := portBackURL(r, slug)
	sep := "?"
	if strings.Contains(next, "?") {
		sep = "&"
	}
	http.Redirect(w, r, next+sep+"port_error="+url.QueryEscape(msg), http.StatusSeeOther)
	return nil
}

func (s *Service) internOptions(r *http.Request, selected []string) ([]pages.InternOption, error) {
	interns, err := s.ListActiveInterns(r.Context())
	if err != nil {
		return nil, err
	}
	selectedSet := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		selectedSet[id] = struct{}{}
	}
	options := make([]pages.InternOption, 0, len(interns))
	for _, intern := range interns {
		_, ok := selectedSet[intern.ID]
		options = append(options, pages.InternOption{
			ID:       intern.ID,
			FullName: intern.FullName,
			Selected: ok,
		})
	}
	return options, nil
}

func (s *Service) ProjectListItems(ctx context.Context, items []ProjectWithInterns) []pages.ProjectListItem {
	out := make([]pages.ProjectListItem, 0, len(items))
	for _, item := range items {
		names := make([]string, 0, len(item.Interns))
		for _, intern := range item.Interns {
			names = append(names, intern.FullName)
		}
		shouldRun := item.Project.ShouldRun == 1
		isLive := item.Project.IsLive == 1
		statusLabel, statusClass := pages.StatusBadge(shouldRun, isLive)
		url := s.ProjectURL(item.Project.Slug)
		mainPort := "—"
		if main, err := s.queries.GetProjectMainPort(ctx, item.Project.ID); err == nil {
			mainPort = strconv.FormatInt(main.Port, 10)
		}
		out = append(out, pages.ProjectListItem{
			Name:            item.Project.Name,
			Slug:            item.Project.Slug,
			Interns:         strings.Join(names, ", "),
			StatusLabel:     statusLabel,
			StatusClass:     statusClass,
			LifecycleStatus: item.Project.LifecycleStatus,
			LifecycleLabel:  pages.LifecycleLabel(item.Project.LifecycleStatus),
			LifecycleClass:  pages.LifecycleClass(item.Project.LifecycleStatus),
			MainPort:        mainPort,
			URL:             url,
			URLLabel:        urlLabel(url),
			UpdatedAt:       formatUpdatedAt(item.Project.UpdatedAt),
		})
	}
	return out
}

func (s *Service) projectDetailData(ctx context.Context, detail ProjectWithInterns, portError string) pages.ProjectDetailData {
	names := make([]string, 0, len(detail.Interns))
	for _, intern := range detail.Interns {
		names = append(names, intern.FullName)
	}
	owners := strings.Join(names, ", ")
	if owners == "" {
		owners = "—"
	}
	shouldRun := detail.Project.ShouldRun == 1
	isLive := detail.Project.IsLive == 1
	statusLabel, statusClass := pages.StatusBadge(shouldRun, isLive)
	url := s.ProjectURL(detail.Project.Slug)
	runtimeIntent := "Stopped intent"
	if shouldRun {
		runtimeIntent = "Should run"
	}
	ports := s.assignedPorts(ctx, detail.Project.ID)
	mainPort := "—"
	for _, item := range ports {
		if item.IsMain {
			mainPort = item.Port
		}
	}
	return pages.ProjectDetailData{
		Path:            "/projects",
		Name:            detail.Project.Name,
		Slug:            detail.Project.Slug,
		Description:     detail.Project.Description,
		Owners:          owners,
		CreatedAt:       formatUpdatedAt(detail.Project.CreatedAt),
		UpdatedAt:       formatUpdatedAt(detail.Project.UpdatedAt),
		Directory:       detail.Project.Directory,
		StartupCommand:  detail.Project.StartupCommand,
		LifecycleStatus: detail.Project.LifecycleStatus,
		LifecycleLabel:  pages.LifecycleLabel(detail.Project.LifecycleStatus),
		LifecycleClass:  pages.LifecycleClass(detail.Project.LifecycleStatus),
		LifecyclePhase:  pages.LifecyclePhase(detail.Project.LifecycleStatus),
		RuntimeIntent:   runtimeIntent,
		ShouldRun:       shouldRun,
		IsLive:          isLive,
		StatusLabel:     statusLabel,
		StatusClass:     statusClass,
		URL:             url,
		URLLabel:        urlLabel(url),
		MainPort:        mainPort,
		AllocatedPorts:  ports,
		PortError:       portError,
		Archived:        detail.Project.LifecycleStatus == "archived",
	}
}

func (s *Service) assignedPorts(ctx context.Context, projectID string) []pages.PortItem {
	rows, err := s.queries.ListProjectPorts(ctx, projectID)
	if err != nil {
		return nil
	}
	live := map[int64]bool{}
	if observed, err := s.queries.ListPortObservations(ctx); err == nil {
		for _, row := range observed {
			live[row.Port] = true
		}
	}
	items := make([]pages.PortItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, pages.PortItem{
			Port:   strconv.FormatInt(row.Port, 10),
			Role:   row.Role,
			IsMain: row.Role == "main",
			Live:   live[row.Port],
		})
	}
	return items
}

func urlLabel(raw string) string {
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return raw
}

func formatUpdatedAt(value string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC().Format("2006-01-02 15:04")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format("2006-01-02 15:04")
	}
	return value
}
