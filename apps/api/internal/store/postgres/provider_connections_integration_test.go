package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/providerconnections"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/secretbox"
	storepostgres "github.com/matspectrum-ai/conver-pay/apps/api/internal/store/postgres"
)

type acceptingValidator struct{}

func (acceptingValidator) ValidateCredentials(context.Context, string, domain.Environment, map[string]string) error {
	return nil
}

type fixedProviderIDs struct{}

func (fixedProviderIDs) New(string) string { return "pc_encrypted" }

func TestProviderConnectionCredentialsAreEncryptedAndAtomic(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error=%v", err)
	}
	defer pool.Close()
	resetDatabase(t, pool)

	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("secretbox.New() error=%v", err)
	}
	store := storepostgres.New(pool)
	svc, err := providerconnections.New(providerconnections.Options{Store: store, Validator: acceptingValidator{}, Cipher: box, IDs: fixedProviderIDs{}})
	if err != nil {
		t.Fatalf("providerconnections.New() error=%v", err)
	}
	connection, err := svc.Connect(ctx, providerconnections.ConnectRequest{WorkspaceID: "ws_1", Environment: domain.EnvironmentTest, ProviderKey: "woovi", Priority: 7, Credentials: map[string]string{"app_id": "woovi-secret-app-id"}})
	if err != nil {
		t.Fatalf("Connect() error=%v", err)
	}
	if connection.ID != "pc_encrypted" || !connection.CredentialsValid {
		t.Fatalf("connection=%#v", connection)
	}

	var ciphertext string
	if err := pool.QueryRow(ctx, `SELECT ciphertext FROM conver_pay.provider_credentials WHERE provider_connection_id=$1`, connection.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("query ciphertext=%v", err)
	}
	if ciphertext == "" || ciphertext == "woovi-secret-app-id" {
		t.Fatalf("unsafe ciphertext=%q", ciphertext)
	}
	plaintext, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error=%v", err)
	}
	var credentials map[string]string
	if err := json.Unmarshal(plaintext, &credentials); err != nil {
		t.Fatalf("unmarshal credentials=%v", err)
	}
	if credentials["app_id"] != "woovi-secret-app-id" {
		t.Fatalf("credentials=%#v", credentials)
	}

	var connectionCount, credentialCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM conver_pay.provider_connections WHERE id=$1`, connection.ID).Scan(&connectionCount)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM conver_pay.provider_credentials WHERE provider_connection_id=$1`, connection.ID).Scan(&credentialCount)
	if connectionCount != 1 || credentialCount != 1 {
		t.Fatalf("connection_count=%d credential_count=%d", connectionCount, credentialCount)
	}
}
