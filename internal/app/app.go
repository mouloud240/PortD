// Package app builds PortD's application dependencies and lifecycle.
package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"sync"
	"time"

	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/auth"
	"github.com/portd/internal/config"
	"github.com/portd/internal/db/generated"
	"github.com/portd/internal/healthcheck"
	apphttp "github.com/portd/internal/http"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/internal/runtime"
	_ "modernc.org/sqlite"
)

// App owns PortD's process-scoped resources.
type App struct {
	database *sql.DB
	handler  http.Handler
	server   *http.Server
	health   *healthcheck.Poller
	runtime  *runtime.Manager
	activity *activitysvc.Service
	cancel   context.CancelFunc
	wait     sync.WaitGroup
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
	runtimeManager := runtime.NewManager()
	activityService := activitysvc.NewService(queries)
	activityService.Start()
	portService := portsvc.NewService(queries, portsvc.NewGopsutilScanner())
	router := apphttp.NewRouter(authService, internService, projectService, portService, activityService, runtimeManager)
	healthPoller, err := healthcheck.NewPoller(queries, nil, healthcheck.WithErrorHandler(func(err error) {
		slog.Error("healthcheck cycle failed", "error", err)
	}))
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	healthContext, cancel := context.WithCancel(context.Background())
	application := &App{
		database: database,
		handler:  router,
		health:   healthPoller,
		runtime:  runtimeManager,
		activity: activityService,
		cancel:   cancel,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
	// Start healthchecks with the application so polling begins before the
	// HTTP server starts accepting requests.
	application.wait.Add(1)
	go func() {
		defer application.wait.Done()
		_ = healthPoller.Run(healthContext)
	}()

	return application, nil
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
	a.cancel()
	runtimeErr := a.runtime.StopAll(ctx)
	a.activity.Close()
	err := a.server.Shutdown(ctx)
	a.wait.Wait()
	if err == nil {
		err = runtimeErr
	}
	return err
}

// Close releases process-scoped resources.
func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
		a.wait.Wait()
	}
	_ = a.runtime.StopAll(context.Background())
	a.activity.Close()
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
	activityService *activitysvc.Service,
) http.Handler {
	return apphttp.NewRouter(authService, internService, projectService, portService, activityService)
}
