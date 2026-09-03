// Package app wires PortD's transport-level dependencies.
package app

import (
	"net/http"

	"github.com/portd/internal/auth"
	internsvc "github.com/portd/internal/interns"
)

// NewHandler returns the root HTTP handler for PortD.
func NewHandler(authService *auth.Service, internService *internsvc.Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", Handle(healthz))
	mux.Handle("GET /login", Handle(authService.LoginPage))
	mux.Handle("POST /login", Handle(authService.LoginPost))
	mux.Handle("POST /logout", Handle(authService.LogoutPost))
	mux.Handle("GET /", Handle(requireSession(authService, home)))
	admin := func(next Handler) Handler { return requireAdmin(authService, next) }
	mux.Handle("GET /interns", Handle(admin(internService.ListPage)))
	mux.Handle("GET /interns/new", Handle(admin(internService.NewPage)))
	mux.Handle("POST /interns", Handle(admin(internService.CreatePost)))
	mux.Handle("GET /interns/{id}", Handle(admin(internService.DetailPage)))
	mux.Handle("POST /interns/{id}", Handle(admin(internService.UpdatePost)))
	return mux
}

func requireSession(service *auth.Service, next Handler) Handler {
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

func requireAdmin(service *auth.Service, next Handler) Handler {
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
			return Forbidden("Administrator access required", nil)
		}
		return next(w, r)
	}
}

func home(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write([]byte("<!doctype html><html><body><h1>PortD</h1><p>Authenticated.</p><form method=post action=/logout><button>Sign out</button></form></body></html>"))
	return err
}

func healthz(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte("ok\n"))
	return err
}
