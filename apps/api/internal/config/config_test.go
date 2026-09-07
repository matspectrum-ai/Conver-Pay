package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("WEBHOOK_SECRET_MASTER_KEY", "")
	t.Setenv("WEBHOOK_WORKER_INTERVAL", "")
	t.Setenv("WEBHOOK_HTTP_TIMEOUT", "")
	t.Setenv("PROVIDER_CREDENTIALS_MASTER_KEY", "")
	t.Setenv("PROVIDER_HTTP_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.WebhookWorkerInterval != time.Second || cfg.WebhookHTTPTimeout != 10*time.Second {
		t.Fatalf("webhook defaults interval=%s timeout=%s", cfg.WebhookWorkerInterval, cfg.WebhookHTTPTimeout)
	}
	if cfg.ProviderHTTPTimeout != 10*time.Second {
		t.Fatalf("provider HTTP timeout = %s", cfg.ProviderHTTPTimeout)
	}
}

func TestProductionRequiresDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("WEBHOOK_SECRET_MASTER_KEY", validWebhookMasterKey())
	t.Setenv("PROVIDER_CREDENTIALS_MASTER_KEY", validWebhookMasterKey())

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestProductionRequiresWebhookMasterKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEBHOOK_SECRET_MASTER_KEY", "")
	t.Setenv("PROVIDER_CREDENTIALS_MASTER_KEY", validWebhookMasterKey())

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "WEBHOOK_SECRET_MASTER_KEY") {
		t.Fatalf("Load() error = %v, want webhook master key error", err)
	}
}

func TestProductionAcceptsRequiredWebhookConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEBHOOK_SECRET_MASTER_KEY", validWebhookMasterKey())
	t.Setenv("PROVIDER_CREDENTIALS_MASTER_KEY", validWebhookMasterKey())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WebhookSecretMasterKey == "" {
		t.Fatal("WebhookSecretMasterKey is empty")
	}
}

func TestProductionRequiresProviderCredentialsMasterKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEBHOOK_SECRET_MASTER_KEY", validWebhookMasterKey())
	t.Setenv("PROVIDER_CREDENTIALS_MASTER_KEY", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PROVIDER_CREDENTIALS_MASTER_KEY") {
		t.Fatalf("Load() error = %v, want provider credentials master key error", err)
	}
}

func validWebhookMasterKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}
