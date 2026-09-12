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

	mux := http.NewServeMux()
	// Misc
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle("GET /healthz", httperr.Handle(pageHandlers.Healthz))
	// Auth
	mux.Handle("GET /login", httperr.Handle(authHandlers.LoginPage))
	mux.Handle("POST /login", httperr.Handle(authHandlers.LoginPost))
	mux.Handle("POST /logout", httperr.Handle(authHandlers.LogoutPost))
	mux.Handle("GET /", httperr.Handle(requireSession(pageHandlers.Dashboard)))
	// Dashboarrd and user pages
	mux.Handle("GET /dashboard", httperr.Handle(requireSession(pageHandlers.Dashboard)))
	mux.Handle("GET /profile", httperr.Handle(requireSession(pageHandlers.ProfilePage)))
	mux.Handle("POST /profile", httperr.Handle(requireSession(pageHandlers.ProfileUpdate)))
	mux.Handle("GET /activity", httperr.Handle(requireAdmin(pageHandlers.Activity)))
	mux.Handle("GET /ports", httperr.Handle(requireSession(portHandlers.ListPage)))
	// Projects

	mux.Handle("GET /projects", httperr.Handle(requireSession(projectHandlers.ListPage)))
	mux.Handle("GET /projects/new", httperr.Handle(requireSession(projectHandlers.NewPage)))
	mux.Handle("POST /projects", httperr.Handle(requireSession(projectHandlers.CreatePost)))
	mux.Handle("GET /projects/{slug}", httperr.Handle(requireProjectMember(projectHandlers.DetailPage)))
	mux.Handle("GET /projects/{slug}/edit", httperr.Handle(requireProjectMember(projectHandlers.EditPage)))
	mux.Handle("POST /projects/{slug}", httperr.Handle(requireProjectMember(projectHandlers.UpdatePost)))
	mux.Handle("POST /projects/{slug}/archive", httperr.Handle(requireProjectMember(projectHandlers.ArchivePost)))
	mux.Handle("POST /projects/{slug}/runtime/start", httperr.Handle(requireProjectMember(projectHandlers.RuntimeStartPost)))
	mux.Handle("POST /projects/{slug}/runtime/stop", httperr.Handle(requireProjectMember(projectHandlers.RuntimeStopPost)))
	mux.Handle("POST /projects/{slug}/runtime/configure", httperr.Handle(requireProjectMember(projectHandlers.RuntimeConfigurePost)))
	mux.Handle("POST /projects/{slug}/ports/promote", httperr.Handle(requireProjectMember(projectHandlers.PromotePortPost)))
	// Ports management
	mux.Handle("POST /projects/{slug}/ports/allocate", httperr.Handle(requireProjectMember(projectHandlers.AllocatePortsPost)))
	mux.Handle("POST /projects/{slug}/ports/release", httperr.Handle(requireProjectMember(projectHandlers.ReleasePortPost)))
	mux.Handle("POST /projects/{slug}/ports/claim", httperr.Handle(requireProjectMember(projectHandlers.ClaimPortPost)))
	// Healthchecks management
	mux.Handle("POST /projects/{slug}/healthchecks", httperr.Handle(requireProjectMember(projectHandlers.AddHealthcheckPost)))
	mux.Handle("POST /projects/{slug}/healthchecks/{id}/delete", httperr.Handle(requireProjectMember(projectHandlers.RemoveHealthcheckPost)))

	// Interns management

	mux.Handle("GET /interns", httperr.Handle(requireAdmin(internHandlers.ListPage)))
	mux.Handle("GET /interns/new", httperr.Handle(requireAdmin(internHandlers.NewPage)))
	mux.Handle("POST /interns", httperr.Handle(requireAdmin(internHandlers.CreatePost)))
	mux.Handle("GET /interns/{id}", httperr.Handle(requireAdmin(internHandlers.DetailPage)))
	mux.Handle("POST /interns/{id}", httperr.Handle(requireAdmin(internHandlers.UpdatePost)))
	return mux
}
