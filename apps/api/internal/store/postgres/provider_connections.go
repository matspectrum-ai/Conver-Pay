package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func (s *Store) CreateProviderConnectionWithCredentials(ctx context.Context, connection domain.ProviderConnection, ciphertext string, validatedAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin provider connection transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO conver_pay.provider_connections (
			id, workspace_id, provider_key, environment, enabled, credentials_valid,
			circuit_state, priority, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)
	`, connection.ID, connection.WorkspaceID, connection.ProviderKey, connection.Environment,
		connection.Enabled, connection.CredentialsValid, connection.Circuit, connection.Priority, validatedAt)
	if err != nil {
		return fmt.Errorf("insert provider connection: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO conver_pay.provider_credentials (
			provider_connection_id, ciphertext, validated_at, created_at, updated_at
		) VALUES ($1,$2,$3,$3,$3)
	`, connection.ID, ciphertext, validatedAt)
	if err != nil {
		return fmt.Errorf("insert provider credentials: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit provider connection: %w", err)
	}
	return nil
}

func (s *Store) GetProviderCredentialCiphertext(ctx context.Context, connectionID string) (string, error) {
	var ciphertext string
	err := s.pool.QueryRow(ctx, `
		SELECT ciphertext FROM conver_pay.provider_credentials WHERE provider_connection_id=$1
	`, connectionID).Scan(&ciphertext)
	if err != nil {
		return "", mapNotFound(err)
	}
	return ciphertext, nil
}

func (s *Store) DisableProviderConnection(ctx context.Context, workspaceID, connectionID string, environment domain.Environment, updatedAt time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE conver_pay.provider_connections
		SET enabled=false, updated_at=$4
		WHERE id=$1 AND workspace_id=$2 AND environment=$3
	`, connectionID, workspaceID, environment, updatedAt)
	if err != nil {
		return fmt.Errorf("disable provider connection: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
