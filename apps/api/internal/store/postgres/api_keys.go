package postgres

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/authn"
)

func (s *Store) ResolveAPIKey(ctx context.Context, token string) (authn.Principal, error) {
	if token == "" {
		return authn.Principal{}, authn.ErrUnauthorized
	}

	hash := sha256.Sum256([]byte(token))
	var principal authn.Principal
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, environment
		FROM conver_pay.api_keys
		WHERE secret_hash = $1 AND status = 'active'
	`, hash[:]).Scan(&principal.APIKeyID, &principal.WorkspaceID, &principal.Environment)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authn.Principal{}, authn.ErrUnauthorized
		}
		return authn.Principal{}, err
	}
	return principal, nil
}
