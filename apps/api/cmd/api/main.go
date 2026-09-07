package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/config"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/database"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/httpapi"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var db *database.DB
	if cfg.DatabaseURL != "" {
		connectCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout)
		db, err = database.Open(connectCtx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()
	} else {
		logger.Warn("DATABASE_URL is not configured; readiness will report unavailable")
	}

	server := httpapi.New(httpapi.Options{
		Addr:            cfg.HTTPAddr,
		Logger:          logger,
		Database:        db,
		ShutdownTimeout: cfg.ShutdownTimeout,
	})

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		logger.Error("http server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("api stopped", "at", time.Now().UTC().Format(time.RFC3339))
}
