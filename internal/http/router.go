// Package http contains PortD's HTTP transport composition.
package http

import (
	"net/http"

	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/auth"
	"github.com/portd/internal/http/handlers"
	"github.com/portd/internal/http/middleware"
	"github.com/portd/internal/httperr"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/internal/runtime"
)

// NewRouter registers PortD's public HTTP routes.
func NewRouter(
	authService *auth.Service,
	internService *internsvc.Service,
	projectService *projectsvc.Service,
	portService *portsvc.Service,
	activityService *activitysvc.Service,
	runtimeManagers ...*runtime.Manager,
) http.Handler {
	pageHandlers := handlers.New(authService, internService, projectService, portService, activityService)
	authHandlers := handlers.NewAuthHandler(authService, activityService)
	internHandlers := handlers.NewInternsHandler(internService, activityService)
	portHandlers := handlers.NewPortsHandler(portService)
	projectHandlers := handlers.NewProjectsHandler(projectService, activityService, runtimeManagers...)
	requireSession := middleware.RequireSession(authService)
	requireAdmin := middleware.RequireAdmin(authService)
	requireProjectMember := middleware.RequireProjectMember(authService, projectService)
	// handle logs one line per request (method/path/status/duration/user)
	// inside httperr.Handle so error statuses are captured.
	handle := func(h httperr.Handler) http.Handler { return httperr.Handle(middleware.RequestLog(h)) }

	mux := http.NewServeMux()
	// Misc
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle("GET /healthz", handle(pageHandlers.Healthz))
	// Auth
	mux.Handle("GET /login", handle(authHandlers.LoginPage))
	mux.Handle("POST /login", handle(authHandlers.LoginPost))
	mux.Handle("POST /logout", handle(authHandlers.LogoutPost))
	mux.Handle("GET /", handle(requireSession(pageHandlers.Dashboard)))
	// Dashboarrd and user pages
	mux.Handle("GET /dashboard", handle(requireSession(pageHandlers.Dashboard)))
	mux.Handle("GET /profile", handle(requireSession(pageHandlers.ProfilePage)))
	mux.Handle("POST /profile", handle(requireSession(pageHandlers.ProfileUpdate)))
	mux.Handle("GET /activity", handle(requireAdmin(pageHandlers.Activity)))
	mux.Handle("GET /ports", handle(requireSession(portHandlers.ListPage)))
	// Projects

	mux.Handle("GET /projects", handle(requireSession(projectHandlers.ListPage)))
	mux.Handle("GET /projects/new", handle(requireSession(projectHandlers.NewPage)))
	mux.Handle("POST /projects", handle(requireSession(projectHandlers.CreatePost)))
	mux.Handle("GET /projects/detect", handle(requireSession(projectHandlers.DetectPage)))
	mux.Handle("POST /projects/detect", handle(requireSession(projectHandlers.DetectPost)))
	mux.Handle("GET /projects/{slug}", handle(requireProjectMember(projectHandlers.DetailPage)))
	mux.Handle("GET /projects/{slug}/edit", handle(requireProjectMember(projectHandlers.EditPage)))
	mux.Handle("POST /projects/{slug}", handle(requireProjectMember(projectHandlers.UpdatePost)))
	mux.Handle("POST /projects/{slug}/archive", handle(requireProjectMember(projectHandlers.ArchivePost)))
	mux.Handle("POST /projects/{slug}/runtime/start", handle(requireProjectMember(projectHandlers.RuntimeStartPost)))
	mux.Handle("POST /projects/{slug}/runtime/stop", handle(requireProjectMember(projectHandlers.RuntimeStopPost)))
	mux.Handle("POST /projects/{slug}/runtime/configure", handle(requireProjectMember(projectHandlers.RuntimeConfigurePost)))
	mux.Handle("POST /projects/{slug}/ports/promote", handle(requireProjectMember(projectHandlers.PromotePortPost)))
	// Ports management
	mux.Handle("POST /projects/{slug}/ports/allocate", handle(requireProjectMember(projectHandlers.AllocatePortsPost)))
	mux.Handle("POST /projects/{slug}/ports/release", handle(requireProjectMember(projectHandlers.ReleasePortPost)))
	mux.Handle("POST /projects/{slug}/ports/claim", handle(requireProjectMember(projectHandlers.ClaimPortPost)))
	mux.Handle("POST /projects/{slug}/ports/replace", handle(requireProjectMember(projectHandlers.ReplaceMainPost)))
	// Healthchecks management
	mux.Handle("POST /projects/{slug}/healthchecks", handle(requireProjectMember(projectHandlers.AddHealthcheckPost)))
	mux.Handle("POST /projects/{slug}/healthchecks/{id}/delete", handle(requireProjectMember(projectHandlers.RemoveHealthcheckPost)))

	// Interns management

	mux.Handle("GET /interns", handle(requireAdmin(internHandlers.ListPage)))
	mux.Handle("GET /interns/new", handle(requireAdmin(internHandlers.NewPage)))
	mux.Handle("POST /interns", handle(requireAdmin(internHandlers.CreatePost)))
	mux.Handle("GET /interns/{id}", handle(requireAdmin(internHandlers.DetailPage)))
	mux.Handle("POST /interns/{id}", handle(requireAdmin(internHandlers.UpdatePost)))
	return mux
}
