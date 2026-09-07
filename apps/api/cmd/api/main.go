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

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/config"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/database"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/httpapi"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/logging"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/orchestration"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/provider"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/secretbox"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/webhookdelivery"
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
	var payments httpapi.PaymentService
	var providerWebhooks httpapi.ProviderWebhookService
	var merchantWebhooks httpapi.MerchantWebhookService
	var webhookWorker *webhookdelivery.Service
	var apiKeys authn.Resolver
	if cfg.DatabaseURL != "" {
		connectCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout)
		db, err = database.Open(connectCtx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		store := storepostgres.New(db.Pool())
		orchestrator := orchestration.New(orchestration.Options{
			Repository: store,
			Providers:  provider.MapRegistry{},
		})
		payments = orchestrator
		providerWebhooks = orchestrator
		apiKeys = store

		if cfg.WebhookSecretMasterKey != "" {
			box, boxErr := secretbox.NewBase64(cfg.WebhookSecretMasterKey)
			if boxErr != nil {
				logger.Error("invalid webhook secret master key", "error", boxErr)
				os.Exit(1)
			}
			allowLocal := cfg.AppEnv == "development" || cfg.AppEnv == "test"
			webhookService, serviceErr := webhookdelivery.New(webhookdelivery.Options{
				Repository: store,
				Cipher:     box,
				Sender: webhookdelivery.NewHTTPSender(webhookdelivery.HTTPOptions{
					Timeout:             cfg.WebhookHTTPTimeout,
					AllowPrivateTargets: allowLocal,
				}),
				AllowInsecureLocalTargets: allowLocal,
			})
			if serviceErr != nil {
				logger.Error("webhook delivery initialization failed", "error", serviceErr)
				os.Exit(1)
			}
			merchantWebhooks = webhookService
			webhookWorker = webhookService
		} else {
			logger.Warn("WEBHOOK_SECRET_MASTER_KEY is not configured; merchant webhook delivery is disabled")
		}
	} else {
		logger.Warn("DATABASE_URL is not configured; readiness will report unavailable")
	}

	server := httpapi.New(httpapi.Options{
		Addr:             cfg.HTTPAddr,
		Logger:           logger,
		Database:         db,
		Payments:         payments,
		ProviderWebhooks: providerWebhooks,
		MerchantWebhooks: merchantWebhooks,
		APIKeys:          apiKeys,
		ShutdownTimeout:  cfg.ShutdownTimeout,
	})

	if webhookWorker != nil {
		go runWebhookWorker(ctx, logger, webhookWorker, cfg.WebhookWorkerInterval)
	}

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

func runWebhookWorker(ctx context.Context, logger *slog.Logger, worker *webhookdelivery.Service, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		processed, err := worker.RunOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("webhook worker iteration failed", "error", err)
		} else if processed > 0 {
			logger.Info("webhook deliveries processed", "count", processed)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
