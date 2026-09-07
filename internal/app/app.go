// Package app wires PortD's transport-level dependencies.
package app

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/views/pages"
)

// NewHandler returns the root HTTP handler for PortD.
func NewHandler(authService *auth.Service, internService *internsvc.Service, projectService *projectsvc.Service, portService *portsvc.Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle("GET /healthz", httperr.Handle(healthz))
	mux.Handle("GET /login", httperr.Handle(authService.LoginPage))
	mux.Handle("POST /login", httperr.Handle(authService.LoginPost))
	mux.Handle("POST /logout", httperr.Handle(authService.LogoutPost))
	mux.Handle("GET /", httperr.Handle(requireSession(authService, dashboard)))
	mux.Handle("GET /dashboard", httperr.Handle(requireSession(authService, dashboard)))
	mux.Handle("GET /profile", httperr.Handle(requireSession(authService, profilePage(authService, internService))))
	mux.Handle("POST /profile", httperr.Handle(requireSession(authService, profileUpdate(authService, internService))))
	for path, title := range map[string]string{"/activity": "Activity"} {
		mux.Handle("GET "+path, httperr.Handle(requireSession(authService, placeholder(title))))
	}
	mux.Handle("GET /ports", httperr.Handle(requireSession(authService, portService.ListPage)))
	admin := func(next httperr.Handler) httperr.Handler { return requireAdmin(authService, next) }
	mux.Handle("GET /projects", httperr.Handle(admin(projectService.ListPage)))
	mux.Handle("GET /projects/new", httperr.Handle(admin(projectService.NewPage)))
	mux.Handle("POST /projects", httperr.Handle(admin(projectService.CreatePost)))
	mux.Handle("GET /projects/{slug}", httperr.Handle(admin(projectService.DetailPage)))
	mux.Handle("GET /projects/{slug}/edit", httperr.Handle(admin(projectService.EditPage)))
	mux.Handle("POST /projects/{slug}", httperr.Handle(admin(projectService.UpdatePost)))
	mux.Handle("POST /projects/{slug}/archive", httperr.Handle(admin(projectService.ArchivePost)))
	mux.Handle("GET /interns", httperr.Handle(admin(internService.ListPage)))
	mux.Handle("GET /interns/new", httperr.Handle(admin(internService.NewPage)))
	mux.Handle("POST /interns", httperr.Handle(admin(internService.CreatePost)))
	mux.Handle("GET /interns/{id}", httperr.Handle(admin(internService.DetailPage)))
	mux.Handle("POST /interns/{id}", httperr.Handle(admin(internService.UpdatePost)))
	return mux
}

func requireSession(service *auth.Service, next httperr.Handler) httperr.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return nil
		}
		if _, err := service.Session(r.Context(), cookie.Value); err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return nil
		}
		return next(w, r)
	}
}

func requireAdmin(service *auth.Service, next httperr.Handler) httperr.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return nil
		}
		principal, err := service.Session(r.Context(), cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return nil
		}
		if !principal.IsAdmin() {
			return httperr.Forbidden("Administrator access required", nil)
		}
		return next(w, r)
	}
}

func dashboard(w http.ResponseWriter, r *http.Request) error {
	return httperr.Render(w, r, http.StatusOK, pages.DashboardPage())
}

func profilePage(authService *auth.Service, internService *internsvc.Service) httperr.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		data, err := loadProfile(r, authService, internService)
		if err != nil {
			return err
		}
		return httperr.Render(w, r, http.StatusOK, pages.ProfilePage(data))
	}
}

func profileUpdate(authService *auth.Service, internService *internsvc.Service) httperr.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		data, err := loadProfile(r, authService, internService)
		if err != nil {
			return err
		}
		if data.IsAdmin {
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return nil
		}
		if err := r.ParseForm(); err != nil {
			return httperr.BadRequest("Invalid form submission.", err)
		}
		data.FullName = r.FormValue("full_name")
		data.Username = r.FormValue("identifier")
		data.Email = r.FormValue("email")
		current, err := internService.Get(r.Context(), principalID(r, authService))
		if err != nil {
			if errors.Is(err, internsvc.ErrNotFound) {
				return httperr.NotFound("Profile not found.", err)
			}
			return err
		}
		if _, err := internService.Update(r.Context(), current.ID, data.FullName, data.Email, data.Username, current.Active == 1); err != nil {
			switch {
			case errors.Is(err, internsvc.ErrInvalid):
				data.Error = "Full name and username are required."
				return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProfilePage(data))
			case errors.Is(err, internsvc.ErrConflict):
				data.Error = "That username or email is already taken."
				return httperr.Render(w, r, http.StatusConflict, pages.ProfilePage(data))
			case errors.Is(err, sql.ErrNoRows):
				return httperr.NotFound("Profile not found.", err)
			default:
				return err
			}
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return nil
	}
}

// principalID resolves the session back to its principal; requireSession
// already gated the request, so failures here are unexpected.
func principalID(r *http.Request, authService *auth.Service) string {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return ""
	}
	principal, err := authService.Session(r.Context(), cookie.Value)
	if err != nil {
		return ""
	}
	return principal.InternID
}

func loadProfile(r *http.Request, authService *auth.Service, internService *internsvc.Service) (pages.ProfileData, error) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return pages.ProfileData{}, httperr.Unauthorized("Sign in required.", err)
	}
	principal, err := authService.Session(r.Context(), cookie.Value)
	if err != nil {
		return pages.ProfileData{}, httperr.Unauthorized("Sign in required.", err)
	}
	if principal.IsAdmin() {
		return pages.ProfileData{Title: "Profile", Path: "/profile", Role: "admin", IsAdmin: true, Username: authService.AdminUsername()}, nil
	}
	intern, err := internService.Get(r.Context(), principal.InternID)
	if err != nil {
		if errors.Is(err, internsvc.ErrNotFound) {
			return pages.ProfileData{}, httperr.NotFound("Profile not found.", err)
		}
		return pages.ProfileData{}, err
	}
	return pages.ProfileData{Title: "Profile", Path: "/profile", Role: "intern", FullName: intern.FullName, Username: intern.Identifier.String, Email: intern.Email.String}, nil
}

func placeholder(title string) httperr.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		return httperr.Render(w, r, http.StatusOK, pages.PlaceholderPage(title, r.URL.Path))
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte("ok\n"))
	return err
}
