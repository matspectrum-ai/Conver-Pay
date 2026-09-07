package postgres_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
)

func TestPostgresAPIKeyResolution(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not configured")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	const token = "cp_test_integration_secret"
	hash := sha256.Sum256([]byte(token))
	_, err = pool.Exec(ctx, `
		INSERT INTO conver_pay.api_keys (id, workspace_id, name, environment, key_prefix, secret_hash, status)
		VALUES ('key_integration', 'ws_integration', 'Integration', 'test', 'cp_test_inte', $1, 'active')
		ON CONFLICT (id) DO UPDATE SET secret_hash=EXCLUDED.secret_hash, status='active', revoked_at=NULL
	`, hash[:])
	if err != nil {
		t.Fatalf("seed API key: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM conver_pay.api_keys WHERE id='key_integration'`)
	})

	store := storepostgres.New(pool)
	principal, err := store.ResolveAPIKey(ctx, token)
	if err != nil {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
	if principal.APIKeyID != "key_integration" || principal.WorkspaceID != "ws_integration" || principal.Environment != domain.EnvironmentTest {
		t.Fatalf("principal = %#v", principal)
	}

	if _, err := store.ResolveAPIKey(ctx, "wrong-secret"); !errors.Is(err, authn.ErrUnauthorized) {
		t.Fatalf("invalid key error = %v, want unauthorized", err)
	}

	_, err = pool.Exec(ctx, `UPDATE conver_pay.api_keys SET status='revoked', revoked_at=now() WHERE id='key_integration'`)
	if err != nil {
		t.Fatalf("revoke API key: %v", err)
	}
	if _, err := store.ResolveAPIKey(ctx, token); !errors.Is(err, authn.ErrUnauthorized) {
		t.Fatalf("revoked key error = %v, want unauthorized", err)
	}
}
