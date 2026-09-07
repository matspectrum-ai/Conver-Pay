package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

func (s *Store) GetProviderConnection(ctx context.Context, id string) (*domain.ProviderConnection, error) {
	var conn domain.ProviderConnection
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, provider_key, environment, enabled,
			credentials_valid, circuit_state, priority
		FROM conver_pay.provider_connections
		WHERE id=$1
	`, id).Scan(&conn.ID, &conn.WorkspaceID, &conn.ProviderKey, &conn.Environment,
		&conn.Enabled, &conn.CredentialsValid, &conn.Circuit, &conn.Priority)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &conn, nil
}

func (s *Store) GetAttemptByProviderPaymentID(ctx context.Context, providerConnectionID, providerPaymentID string) (*domain.PaymentAttempt, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+attemptColumns+`
		FROM conver_pay.payment_attempts
		WHERE provider_connection_id=$1 AND provider_payment_id=$2
		ORDER BY sequence DESC LIMIT 1`, providerConnectionID, providerPaymentID)
	attempt, err := scanAttempt(row)
	return attempt, mapNotFound(err)
}

func (s *Store) RecordProviderEvent(ctx context.Context, event *domain.ProviderEvent) (bool, error) {
	result, err := s.pool.Exec(ctx, `
		INSERT INTO conver_pay.provider_events (
			id, provider_connection_id, external_event_id, event_type,
			provider_payment_id, payload_hash, received_at, processed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (provider_connection_id, external_event_id) DO NOTHING`,
		event.ID, event.ProviderConnectionID, event.ExternalEventID, event.EventType,
		event.ProviderPaymentID, event.PayloadHash, event.ReceivedAt, event.ProcessedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert provider event: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *Store) MarkProviderEventProcessed(ctx context.Context, providerConnectionID, externalEventID string, processedAt time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE conver_pay.provider_events
		SET processed_at=$3
		WHERE provider_connection_id=$1 AND external_event_id=$2`,
		providerConnectionID, externalEventID, processedAt)
	if err != nil {
		return fmt.Errorf("mark provider event processed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) MarkPaymentPaidWithEvents(ctx context.Context, intent *domain.PaymentIntent, recovery *domain.RecoveryEvent, events []domain.MerchantEvent) error {
	metadata, err := json.Marshal(intent.Metadata)
	if err != nil {
		return fmt.Errorf("marshal payment metadata: %w", err)
	}
	pixCopy, pixExpires := pixValues(intent.Pix)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin paid transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		UPDATE conver_pay.payment_intents SET
			merchant_order_id=$2, request_fingerprint=$3, amount=$4, currency=$5,
			metadata=$6, status=$7, active_attempt_id=$8, presented_attempt_id=$9,
			pix_copy_paste=$10, pix_expires_at=$11, recovered=$12, recovered_amount=$13,
			failure_code=$14, updated_at=$15, paid_at=$16
		WHERE id=$1`,
		intent.ID, intent.MerchantOrderID, intent.RequestFingerprint, intent.Amount,
		intent.Currency, metadata, intent.Status, intent.ActiveAttemptID,
		intent.PresentedAttemptID, pixCopy, pixExpires, intent.Recovered,
		intent.RecoveredAmount, intent.FailureCode, intent.UpdatedAt, intent.PaidAt,
	)
	if err != nil {
		return fmt.Errorf("update paid payment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if recovery != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO conver_pay.recovery_events (
				id, payment_intent_id, failed_attempt_id, successful_attempt_id,
				failure_reason, amount, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (payment_intent_id) DO NOTHING`,
			recovery.ID, recovery.PaymentIntentID, recovery.FailedAttemptID,
			recovery.SuccessfulAttemptID, recovery.FailureReason, recovery.Amount, recovery.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert paid recovery event: %w", err)
		}
	}

	for _, event := range events {
		_, err = tx.Exec(ctx, `
			INSERT INTO conver_pay.merchant_events (
				id, workspace_id, event_key, event_type, payment_intent_id, payload, created_at
			) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)
			ON CONFLICT (event_key) DO NOTHING`,
			event.ID, event.WorkspaceID, event.EventKey, event.EventType,
			event.PaymentIntentID, string(event.Payload), event.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert merchant event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit paid transaction: %w", err)
	}
	return nil
}

func (s *Store) ListMerchantEvents(ctx context.Context, workspaceID string) ([]domain.MerchantEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, event_key, event_type, payment_intent_id, payload, created_at
		FROM conver_pay.merchant_events
		WHERE workspace_id=$1 ORDER BY created_at, id`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list merchant events: %w", err)
	}
	defer rows.Close()

	var events []domain.MerchantEvent
	for rows.Next() {
		var event domain.MerchantEvent
		if err := rows.Scan(&event.ID, &event.WorkspaceID, &event.EventKey, &event.EventType,
			&event.PaymentIntentID, &event.Payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan merchant event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate merchant events: %w", err)
	}
	return events, nil
}
