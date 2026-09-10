package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/portd/internal/auth"
	db "github.com/portd/internal/db/generated"
	"github.com/portd/internal/httperr"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/views/pages"
)

func (h *ProjectsHandler) ListPage(w http.ResponseWriter, r *http.Request) error {
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
	items, err := h.scopedList(r, search, lifecycle, isLive)
	if err != nil {
		if errors.Is(err, projectsvc.ErrInvalid) {
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
		h.ProjectListItems(r.Context(), items),
	))
}

func (h *ProjectsHandler) scopedList(r *http.Request, search, lifecycle string, isLive int64) ([]projectsvc.ProjectWithInterns, error) {
	principal, _ := auth.PrincipalFrom(r.Context())
	if principal.IsAdmin() {
		return h.service.List(r.Context(), search, lifecycle, isLive)
	}
	if principal.InternID == "" {
		return nil, projectsvc.ErrInvalid
	}
	return h.service.ListForIntern(r.Context(), principal.InternID, search, lifecycle, isLive)
}

func (h *ProjectsHandler) NewPage(w http.ResponseWriter, r *http.Request) error {
	selected := []string{}
	if principal, _ := auth.PrincipalFrom(r.Context()); !principal.IsAdmin() && principal.InternID != "" {
		selected = []string{principal.InternID}
	}
	options, err := h.internOptions(r, selected)
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
		AccessMode:      projectsvc.AccessModeDirect,
		Interns:         options,
	}))
}

func (h *ProjectsHandler) CreatePost(w http.ResponseWriter, r *http.Request) error {
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
		AccessMode:      r.FormValue("access_mode"),
	}
	internIDs := r.Form["intern_ids"]
	options, err := h.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	creator := ""
	if principal, _ := auth.PrincipalFrom(r.Context()); !principal.IsAdmin() {
		creator = principal.InternID
	}
	created, err := h.service.Create(r.Context(), projectsvc.CreateInput{
		Name:            form.Name,
		Slug:            form.Slug,
		Description:     form.Description,
		InternIDs:       internIDs,
		LifecycleStatus: form.LifecycleStatus,
		ShouldRun:       form.ShouldRun,
		PortCount:       portCount,
		CreatorInternID: creator,
		AccessMode:      form.AccessMode,
	})
	if err != nil {
		switch {
		case errors.Is(err, projectsvc.ErrInvalid):
			form.Error = "Name, at least one active intern, a valid lifecycle, and 1–5 ports are required."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProjectFormPage(form))
		case errors.Is(err, projectsvc.ErrConflict):
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

func (h *ProjectsHandler) DetailPage(w http.ResponseWriter, r *http.Request) error {
	detail, err := h.service.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Project not found.", err)
		}
		return err
	}
	data, err := h.projectDetailData(r.Context(), detail, r.URL.Query().Get("port_error"), r.URL.Query().Get("health_error"))
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectDetailPage(data))
}

func (h *ProjectsHandler) EditPage(w http.ResponseWriter, r *http.Request) error {
	detail, err := h.service.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Project not found.", err)
		}
		return err
	}
	selected := make([]string, 0, len(detail.Interns))
	for _, intern := range detail.Interns {
		selected = append(selected, intern.ID)
	}
	options, err := h.internOptions(r, selected)
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
		AccessMode:      detail.Project.AccessMode,
		Ports:           h.assignedPorts(r.Context(), detail.Project.ID),
		PortError:       r.URL.Query().Get("port_error"),
		Interns:         options,
	}))
}

func (h *ProjectsHandler) UpdatePost(w http.ResponseWriter, r *http.Request) error {
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
		AccessMode:      r.FormValue("access_mode"),
	}
	internIDs := r.Form["intern_ids"]
	options, err := h.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	if projectID, err := h.service.GetBySlug(r.Context(), slug); err == nil {
		form.Ports = h.assignedPorts(r.Context(), projectID.Project.ID)
	}

	updated, err := h.service.Update(r.Context(), slug, projectsvc.UpdateInput{
		Name:            form.Name,
		Description:     form.Description,
		InternIDs:       internIDs,
		LifecycleStatus: form.LifecycleStatus,
		ShouldRun:       form.ShouldRun,
		AccessMode:      form.AccessMode,
	})
	if err != nil {
		switch {
		case errors.Is(err, projectsvc.ErrInvalid):
			form.Error = "Name, at least one active intern, and a valid lifecycle are required."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProjectFormPage(form))
		case errors.Is(err, projectsvc.ErrNotFound):
			return httperr.NotFound("Project not found.", err)
		default:
			return err
		}
	}
	form.IsLive = updated.Project.IsLive == 1
	http.Redirect(w, r, "/projects/"+updated.Project.Slug, http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) ArchivePost(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	archived, err := h.service.Archive(r.Context(), slug)
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Project not found.", err)
		}
		return err
	}
	http.Redirect(w, r, "/projects/"+archived.Project.Slug, http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) healthBack(w http.ResponseWriter, r *http.Request, slug string, err error) error {
	if errors.Is(err, projectsvc.ErrNotFound) {
		return httperr.NotFound("Project not found.", err)
	}
	msg := "Healthcheck action failed."
	if errors.Is(err, projectsvc.ErrInvalid) {
		msg = "Endpoint must start with / or http(s):// and expect status 200–599."
	} else {
		return err
	}
	next := portBackURL(r, slug)
	sep := "?"
	if strings.Contains(next, "?") {
		sep = "&"
	}
	http.Redirect(w, r, next+sep+"health_error="+url.QueryEscape(msg), http.StatusSeeOther)
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

func (h *ProjectsHandler) portBack(w http.ResponseWriter, r *http.Request, slug string, err error) error {
	if errors.Is(err, projectsvc.ErrNotFound) || errors.Is(err, portsvc.ErrNotFound) {
		return httperr.NotFound("Project not found.", err)
	}
	msg := "Port action failed."
	switch {
	case errors.Is(err, projectsvc.ErrInvalid):
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

func (h *ProjectsHandler) internOptions(r *http.Request, selected []string) ([]pages.InternOption, error) {
	interns, err := h.service.ListActiveInterns(r.Context())
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

func (h *ProjectsHandler) ProjectListItems(ctx context.Context, items []projectsvc.ProjectWithInterns) []pages.ProjectListItem {
	out := make([]pages.ProjectListItem, 0, len(items))
	for _, item := range items {
		names := make([]string, 0, len(item.Interns))
		for _, intern := range item.Interns {
			names = append(names, intern.FullName)
		}
		shouldRun := item.Project.ShouldRun == 1
		isLive := item.Project.IsLive == 1
		statusLabel, statusClass := pages.StatusBadge(shouldRun, isLive)
		access := h.projectAccess(ctx, item.Project)
		mainPort := "—"
		if main, err := h.service.MainPort(ctx, item.Project.ID); err == nil {
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
			URL:             access.URL,
			URLLabel:        access.URLLabel,
			AccessMode:      access.Mode,
			AccessLabel:     access.ModeLabel,
			DirectURL:       access.DirectURL,
			DirectURLLabel:  access.DirectURLLabel,
			ProxiedURL:      access.ProxiedURL,
			ProxiedURLLabel: access.ProxiedURLLabel,
			UpdatedAt:       formatUpdatedAt(item.Project.UpdatedAt),
		})
	}
	return out
}

func (h *ProjectsHandler) projectDetailData(ctx context.Context, detail projectsvc.ProjectWithInterns, portError, healthError string) (pages.ProjectDetailData, error) {
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
	access := h.projectAccess(ctx, detail.Project)
	runtimeIntent := "Stopped intent"
	if shouldRun {
		runtimeIntent = "Should run"
	}
	ports := h.assignedPorts(ctx, detail.Project.ID)
	mainPort := "—"
	for _, item := range ports {
		if item.IsMain {
			mainPort = item.Port
		}
	}
	checks, err := h.service.ListHealthchecks(ctx, detail.Project.ID)
	if err != nil {
		return pages.ProjectDetailData{}, err
	}
	endpoints := make([]pages.HealthcheckItem, 0, len(checks))
	for _, check := range checks {
		endpoints = append(endpoints, pages.HealthcheckItem{
			ID:       check.ID,
			Endpoint: check.Endpoint,
			Expected: strconv.FormatInt(check.ExpectedStatus, 10),
		})
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
		URL:             access.URL,
		URLLabel:        access.URLLabel,
		Access:          access,
		MainPort:        mainPort,
		AllocatedPorts:  ports,
		PortError:       portError,
		Healthchecks:    endpoints,
		HealthError:     healthError,
		Archived:        detail.Project.LifecycleStatus == "archived",
	}, nil
}

func (h *ProjectsHandler) projectAccess(ctx context.Context, project db.Project) pages.ProjectAccessData {
	main, err := h.service.MainPort(ctx, project.ID)
	if err != nil {
		return pages.ProjectAccessData{Mode: project.AccessMode, ModeLabel: accessModeLabel(project.AccessMode)}
	}
	direct, err := h.service.DirectProjectURL(main.Port)
	if err != nil {
		direct = ""
	}
	proxied := h.service.ProjectURL(project.Slug)
	access := pages.ProjectAccessData{
		Mode:            project.AccessMode,
		ModeLabel:       accessModeLabel(project.AccessMode),
		DirectURL:       direct,
		DirectURLLabel:  urlLabel(direct),
		ProxiedURL:      proxied,
		ProxiedURLLabel: urlLabel(proxied),
		Quickstarts:     quickstarts(proxied),
	}
	access.URL, access.URLLabel = access.ProxiedURL, access.ProxiedURLLabel
	if project.AccessMode == projectsvc.AccessModeDirect {
		access.URL, access.URLLabel = access.DirectURL, access.DirectURLLabel
	}
	return access
}

func accessModeLabel(mode string) string {
	if mode == projectsvc.AccessModeProxied {
		return "Proxied"
	}
	return "Direct"
}

func quickstarts(proxied string) []pages.QuickstartItem {
	base := proxied
	if parsed, err := url.Parse(proxied); err == nil && parsed.Path != "" {
		base = strings.TrimRight(parsed.Path, "/")
	}
	return []pages.QuickstartItem{
		{Name: "React Router", Description: "Pass the project path as your router basename.", Snippet: `<BrowserRouter basename="` + base + `">`},
		{Name: "Vite", Description: "Set the base so bundled assets resolve under the project path.", Snippet: "base: '" + base + "/'"},
		{Name: "Next.js", Description: "Set basePath once for links and assets.", Snippet: "basePath: '" + base + "'"},
		{Name: "Vue Router", Description: "Pass the project path to history mode.", Snippet: "createWebHistory('" + base + "/')"},
		{Name: "Nuxt", Description: "Set baseURL for routing and assets.", Snippet: "app: { baseURL: '" + base + "/' }"},
		{Name: "Angular", Description: "Set the document base for router and asset URLs.", Snippet: `<base href="` + base + `/">`},
		{Name: "SvelteKit", Description: "Set the adapter base path.", Snippet: "paths: { base: '" + base + "' }"},
		{Name: "Hash routing", Description: "Hash routes already work under a path prefix.", Snippet: "No configuration needed", NoConfig: true},
	}
}

func (h *ProjectsHandler) assignedPorts(ctx context.Context, projectID string) []pages.PortItem {
	rows, err := h.service.ListPorts(ctx, projectID)
	if err != nil {
		return nil
	}
	live := map[int64]bool{}
	if observed, err := h.service.ListPortObservations(ctx); err == nil {
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
