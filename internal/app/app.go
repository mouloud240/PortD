// Package app wires PortD's transport-level dependencies.
package app

import (
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"

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

func dashboard(w http.ResponseWriter, r *http.Request) error {
	return renderPlaceholder(w, "Overview", r.URL.Path)
}

func placeholder(title string) Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		return renderPlaceholder(w, title, r.URL.Path)
	}
}

type placeholderData struct {
	Title string
	Path  string
}

var placeholderTemplate = template.Must(template.New("shell.html").Funcs(template.FuncMap{"active": func(current, path string) string {
	if current == path {
		return "active"
	}
	return ""
}}).ParseFiles(appViewPath("views/layouts/shell.html"), appViewPath("views/pages/placeholder.html")))

func renderPlaceholder(w http.ResponseWriter, title string, path string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return placeholderTemplate.ExecuteTemplate(w, "placeholder", placeholderData{Title: title, Path: path})
}

func appViewPath(path string) string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, path)
}

func healthz(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte("ok\n"))
	return err
}
