package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/portd/internal/app"
	"github.com/portd/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg := config.Load()
	logger.Info("loaded configuration", "addr", cfg.HTTPAddr, "db", cfg.DBPath, "projects_dir", cfg.ProjectsDir, "base_url", cfg.BaseURL)
	application, err := app.New(cfg)
	if err != nil {
		logger.Error("build application", "error", err)
		return
	}
	defer application.Close()

	go func() {
		logger.Info("starting PortD", "address", cfg.HTTPAddr)
		if err := application.ListenAndServe(cfg.HTTPAddr); err != nil && !errors.Is(err, app.ErrServerClosed) {
			logger.Error("PortD stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-signalContext.Done()
	logger.Info("shutdown signal received")

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return
	}
	logger.Info("PortD stopped")
}
