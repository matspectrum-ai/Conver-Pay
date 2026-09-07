package postgres

import (
	"context"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func (s *Store) GetPaymentEnvironment(ctx context.Context, paymentID string) (domain.Environment, error) {
	var environment domain.Environment
	err := s.pool.QueryRow(ctx, `
		SELECT pc.environment
		FROM conver_pay.payment_attempts pa
		JOIN conver_pay.provider_connections pc ON pc.id = pa.provider_connection_id
		WHERE pa.payment_intent_id = $1
		ORDER BY pa.sequence ASC
		LIMIT 1
	`, paymentID).Scan(&environment)
	if err != nil {
		return "", mapNotFound(err)
	}
	return environment, nil
}
