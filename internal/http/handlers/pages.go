package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/views/pages"
)

// Handlers contains the transport handlers that compose multiple services.
type Handlers struct {
	auth     *auth.Service
	intern   *internsvc.Service
	project  *projectsvc.Service
	port     *portsvc.Service
	activity *activitysvc.Service
}

func New(
	authService *auth.Service,
	internService *internsvc.Service,
	projectService *projectsvc.Service,
	portService *portsvc.Service,
	activityService *activitysvc.Service,
) *Handlers {
	return &Handlers{
		auth:     authService,
		intern:   internService,
		project:  projectService,
		port:     portService,
		activity: activityService,
	}
}

func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	overview, err := h.project.Overview(ctx)
	if err != nil {
		return err
	}
	interns, err := h.intern.List(ctx, "", 1)
	if err != nil {
		return err
	}
	assigned, unknown, err := h.port.Stats(ctx)
	if err != nil {
		return err
	}
	lifecycle := make([]pages.LifecycleBar, 0, len(overview.Lifecycle))
	for _, entry := range overview.Lifecycle {
		lifecycle = append(lifecycle, pages.LifecycleBar{
			Status:  entry.Status,
			Label:   entry.Label,
			Class:   entry.Class,
			Color:   entry.Color,
			Count:   entry.Count,
			Percent: entry.Percent,
		})
	}
	return httperr.Render(w, r, http.StatusOK, pages.DashboardPage(pages.DashboardData{
		ActiveProjects: overview.Active,
		LiveServices:   overview.Live,
		TotalServices:  overview.Active,
		AssignedPorts:  assigned,
		UnknownPorts:   unknown,
		ActiveInterns:  len(interns),
		Lifecycle:      lifecycle,
		Projects:       NewProjectsHandler(h.project, h.activity).ProjectListItems(ctx, overview.Recent),
		Recent:         h.recentActivity(r),
	}))
}

func (h *Handlers) ProfilePage(w http.ResponseWriter, r *http.Request) error {
	data, err := h.loadProfile(r)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProfilePage(data))
}

func (h *Handlers) ProfileUpdate(w http.ResponseWriter, r *http.Request) error {
	data, err := h.loadProfile(r)
	if err != nil {
		return err
	}
	if data.IsAdmin {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	data.FullName = r.FormValue("full_name")
	data.Username = r.FormValue("identifier")
	data.Email = r.FormValue("email")
	current, err := h.intern.Get(r.Context(), h.principalID(r))
	if err != nil {
		if errors.Is(err, internsvc.ErrNotFound) {
			return httperr.NotFound("Profil introuvable.", err)
		}
		return err
	}
	if _, err := h.intern.Update(r.Context(), current.ID, data.FullName, data.Email, data.Username, current.Active == 1); err != nil {
		switch {
		case errors.Is(err, internsvc.ErrInvalid):
			data.Error = "Le nom complet et le nom d'utilisateur sont requis."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProfilePage(data))
		case errors.Is(err, internsvc.ErrConflict):
			data.Error = "Ce nom d'utilisateur ou cet e-mail est déjà pris."
			return httperr.Render(w, r, http.StatusConflict, pages.ProfilePage(data))
		case errors.Is(err, sql.ErrNoRows):
			return httperr.NotFound("Profil introuvable.", err)
		default:
			return err
		}
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.ProfileUpdate,
		EntityType: "intern",
		EntityID:   current.ID,
		Detail:     data.FullName,
	})
	return nil
}

func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte("ok\n"))
	return err
}

func (h *Handlers) principalID(r *http.Request) string {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return ""
	}
	return principal.InternID
}

func (h *Handlers) loadProfile(r *http.Request) (pages.ProfileData, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return pages.ProfileData{}, httperr.Unauthorized("Connexion requise.", nil)
	}
	if principal.IsAdmin() {
		return pages.ProfileData{
			Title:    "Profil",
			Path:     "/profile",
			Role:     "administrateur",
			IsAdmin:  true,
			Username: h.auth.AdminUsername(),
		}, nil
	}
	intern, err := h.intern.Get(r.Context(), principal.InternID)
	if err != nil {
		if errors.Is(err, internsvc.ErrNotFound) {
			return pages.ProfileData{}, httperr.NotFound("Profil introuvable.", err)
		}
		return pages.ProfileData{}, err
	}
	return pages.ProfileData{
		Title:    "Profil",
		Path:     "/profile",
		Role:     "stagiaire",
		FullName: intern.FullName,
		Username: intern.Identifier.String,
		Email:    intern.Email.String,
	}, nil
}
