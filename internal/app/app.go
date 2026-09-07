// Package app builds PortD's application dependencies and lifecycle.
package app

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/config"
	"github.com/portd/internal/db/generated"
	apphttp "github.com/portd/internal/http"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	_ "modernc.org/sqlite"
)

// App owns PortD's process-scoped resources.
type App struct {
	database *sql.DB
	handler  http.Handler
	server   *http.Server
}

// New constructs the database, services, and HTTP router for PortD.
func New(cfg config.Config) (*App, error) {
	database, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = database.Close()
		return nil, err
	}

	queries := db.New(database)
	authService := auth.NewService(queries, cfg.AdminUsername, cfg.AdminPassword)
	internService := internsvc.NewService(queries)
	projectService := projectsvc.NewService(database, queries, cfg.BaseURL)
	portService := portsvc.NewService(queries, portsvc.NewGopsutilScanner())
	router := apphttp.NewRouter(authService, internService, projectService, portService)

	return &App{
		database: database,
		handler:  router,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// Handler returns the application's HTTP handler.
func (a *App) Handler() http.Handler {
	return a.handler
}

// ListenAndServe starts the configured HTTP server.
func (a *App) ListenAndServe(addr string) error {
	a.server.Addr = addr
	return a.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (a *App) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

// Close releases process-scoped resources.
func (a *App) Close() error {
	return a.database.Close()
}

// ErrServerClosed is returned when the HTTP server has been shut down.
var ErrServerClosed = http.ErrServerClosed

// NewHandler builds a router from explicitly provided services.
//
// It remains as a small compatibility seam for tests and callers that provide
// their own implementations.
func NewHandler(
	authService *auth.Service,
	internService *internsvc.Service,
	projectService *projectsvc.Service,
	portService *portsvc.Service,
) http.Handler {
	return apphttp.NewRouter(authService, internService, projectService, portService)
}
