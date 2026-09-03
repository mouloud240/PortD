package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/portd/internal/app"
	"github.com/portd/internal/auth"
	"github.com/portd/internal/config"
	"github.com/portd/internal/db/generated"
	_ "modernc.org/sqlite"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	database, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		logger.Error("open database", "error", err)
		return
	}
	defer database.Close()
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		logger.Error("enable foreign keys", "error", err)
		return
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.NewHandler(auth.NewService(db.New(database), cfg.AdminUsername, cfg.AdminPassword)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting PortD", "address", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("PortD stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-signalContext.Done()

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
