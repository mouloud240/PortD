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
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle("GET /healthz", Handle(healthz))
	mux.Handle("GET /login", Handle(authService.LoginPage))
	mux.Handle("POST /login", Handle(authService.LoginPost))
	mux.Handle("POST /logout", Handle(authService.LogoutPost))
	mux.Handle("GET /", Handle(requireSession(authService, dashboard)))
	mux.Handle("GET /dashboard", Handle(requireSession(authService, dashboard)))
	for path, title := range map[string]string{"/projects": "All projects", "/projects/tracking": "Project tracking", "/projects/new": "Add project", "/ports": "Port table", "/activity": "Activity"} {
		mux.Handle("GET "+path, Handle(requireSession(authService, placeholder(title))))
	}
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

func dashboard(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write([]byte("<!doctype html><html><head><meta charset=utf-8><meta name=viewport content=width=device-width,initial-scale=1><title>Overview · PortD</title><script src=https://cdn.tailwindcss.com></script></head><body class=bg-slate-50><main class='mx-auto max-w-4xl p-8'><h1 class='text-2xl font-extrabold text-slate-900'>Overview</h1><p class='mt-2 text-slate-600'>Project overview will appear here.</p><form method=post action=/logout class='mt-6'><button class='rounded bg-blue-700 px-4 py-2 text-sm font-bold text-white'>Sign out</button></form></main></body></html>"))
	return err
}

func placeholder(title string) Handler {
	return func(w http.ResponseWriter, _ *http.Request) error {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte("<!doctype html><html><head><meta charset=utf-8><meta name=viewport content=width=device-width,initial-scale=1><title>" + title + " · PortD</title><script src=https://cdn.tailwindcss.com></script></head><body class=bg-slate-50><main class='mx-auto max-w-4xl p-8'><h1 class='text-2xl font-extrabold text-slate-900'>" + title + "</h1></main></body></html>"))
		return err
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte("ok\n"))
	return err
}
