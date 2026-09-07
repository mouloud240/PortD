package projects

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/portd/internal/httperr"
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
	items, err := s.List(r.Context(), search, lifecycle, isLive)
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
		s.projectListItems(items),
	))
}

func (s *Service) NewPage(w http.ResponseWriter, r *http.Request) error {
	options, err := s.internOptions(r, nil)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectFormPage(pages.ProjectFormData{
		Title:           "New project",
		Path:            "/projects",
		Action:          "/projects",
		LifecycleStatus: "draft",
		IsNew:           true,
		Interns:         options,
	}))
}

func (s *Service) CreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	shouldRun, _ := strconv.ParseBool(r.FormValue("should_run"))
	form := pages.ProjectFormData{
		Title:           "New project",
		Path:            "/projects",
		Action:          "/projects",
		Name:            r.FormValue("name"),
		Slug:            r.FormValue("slug"),
		Description:     r.FormValue("description"),
		LifecycleStatus: r.FormValue("lifecycle_status"),
		ShouldRun:       shouldRun,
		IsNew:           true,
	}
	internIDs := r.Form["intern_ids"]
	options, err := s.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	created, err := s.Create(r.Context(), CreateInput{
		Name:            form.Name,
		Slug:            form.Slug,
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
		case errors.Is(err, ErrConflict):
			form.Error = "A project with that slug already exists."
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
	return httperr.Render(w, r, http.StatusOK, pages.ProjectDetailPage(s.projectDetailData(detail)))
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

func (s *Service) projectListItems(items []ProjectWithInterns) []pages.ProjectListItem {
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
		out = append(out, pages.ProjectListItem{
			Name:            item.Project.Name,
			Slug:            item.Project.Slug,
			Interns:         strings.Join(names, ", "),
			StatusLabel:     statusLabel,
			StatusClass:     statusClass,
			LifecycleStatus: item.Project.LifecycleStatus,
			LifecycleLabel:  pages.LifecycleLabel(item.Project.LifecycleStatus),
			LifecycleClass:  pages.LifecycleClass(item.Project.LifecycleStatus),
			MainPort:        "—",
			URL:             url,
			URLLabel:        urlLabel(url),
			UpdatedAt:       formatUpdatedAt(item.Project.UpdatedAt),
		})
	}
	return out
}

func (s *Service) projectDetailData(detail ProjectWithInterns) pages.ProjectDetailData {
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
		MainPort:        "—",
		Archived:        detail.Project.LifecycleStatus == "archived",
	}
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
