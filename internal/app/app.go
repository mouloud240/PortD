// Package app wires PortD's transport-level dependencies.
package app

import (
	"net/http"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
	internsvc "github.com/portd/internal/interns"
	"github.com/portd/views/pages"
)

// NewHandler returns the root HTTP handler for PortD.
func NewHandler(authService *auth.Service, internService *internsvc.Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle("GET /healthz", httperr.Handle(healthz))
	mux.Handle("GET /login", httperr.Handle(authService.LoginPage))
	mux.Handle("POST /login", httperr.Handle(authService.LoginPost))
	mux.Handle("POST /logout", httperr.Handle(authService.LogoutPost))
	mux.Handle("GET /", httperr.Handle(requireSession(authService, dashboard)))
	mux.Handle("GET /dashboard", httperr.Handle(requireSession(authService, dashboard)))
	for path, title := range map[string]string{"/projects": "All projects", "/projects/tracking": "Project tracking", "/projects/new": "Add project", "/ports": "Port table", "/activity": "Activity"} {
		mux.Handle("GET "+path, httperr.Handle(requireSession(authService, placeholder(title))))
	}
	admin := func(next httperr.Handler) httperr.Handler { return requireAdmin(authService, next) }
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
