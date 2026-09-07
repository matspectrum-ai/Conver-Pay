package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/webhookdelivery"
)

func (s *Store) UpsertWebhookEndpoint(ctx context.Context, endpoint *webhookdelivery.Endpoint) (*webhookdelivery.Endpoint, error) {
	var saved webhookdelivery.Endpoint
	err := s.pool.QueryRow(ctx, `
		INSERT INTO conver_pay.merchant_webhook_endpoints (
			id, workspace_id, environment, url, signing_secret_ciphertext,
			enabled, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (workspace_id, environment) DO UPDATE SET
			url=EXCLUDED.url,
			signing_secret_ciphertext=EXCLUDED.signing_secret_ciphertext,
			enabled=EXCLUDED.enabled,
			updated_at=EXCLUDED.updated_at
		RETURNING id, workspace_id, environment, url, signing_secret_ciphertext,
			enabled, created_at, updated_at
	`, endpoint.ID, endpoint.WorkspaceID, endpoint.Environment, endpoint.URL,
		endpoint.SigningSecretCiphertext, endpoint.Enabled, endpoint.CreatedAt, endpoint.UpdatedAt,
	).Scan(&saved.ID, &saved.WorkspaceID, &saved.Environment, &saved.URL,
		&saved.SigningSecretCiphertext, &saved.Enabled, &saved.CreatedAt, &saved.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert webhook endpoint: %w", err)
	}
	return &saved, nil
}

func (s *Store) GetWebhookEndpoint(ctx context.Context, workspaceID string, environment domain.Environment) (*webhookdelivery.Endpoint, error) {
	var endpoint webhookdelivery.Endpoint
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, environment, url, signing_secret_ciphertext,
			enabled, created_at, updated_at
		FROM conver_pay.merchant_webhook_endpoints
		WHERE workspace_id=$1 AND environment=$2
	`, workspaceID, environment).Scan(&endpoint.ID, &endpoint.WorkspaceID, &endpoint.Environment,
		&endpoint.URL, &endpoint.SigningSecretCiphertext, &endpoint.Enabled,
		&endpoint.CreatedAt, &endpoint.UpdatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &endpoint, nil
}

func (s *Store) DisableWebhookEndpoint(ctx context.Context, workspaceID string, environment domain.Environment, updatedAt time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE conver_pay.merchant_webhook_endpoints
		SET enabled=false, updated_at=$3
		WHERE workspace_id=$1 AND environment=$2
	`, workspaceID, environment, updatedAt)
	if err != nil {
		return fmt.Errorf("disable webhook endpoint: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) MaterializePendingDeliveries(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	result, err := s.pool.Exec(ctx, `
		WITH candidates AS (
			SELECT me.id AS merchant_event_id, endpoint.id AS webhook_endpoint_id
			FROM conver_pay.merchant_events me
			JOIN LATERAL (
				SELECT pc.environment
				FROM conver_pay.payment_attempts pa
				JOIN conver_pay.provider_connections pc ON pc.id=pa.provider_connection_id
				WHERE pa.payment_intent_id=me.payment_intent_id
				ORDER BY pa.sequence ASC
				LIMIT 1
			) scope ON true
			JOIN conver_pay.merchant_webhook_endpoints endpoint
				ON endpoint.workspace_id=me.workspace_id
				AND endpoint.environment=scope.environment
				AND endpoint.enabled=true
			WHERE me.created_at >= endpoint.created_at
				AND NOT EXISTS (
					SELECT 1 FROM conver_pay.webhook_deliveries existing
					WHERE existing.merchant_event_id=me.id
				)
			ORDER BY me.created_at, me.id
			LIMIT $2
		)
		INSERT INTO conver_pay.webhook_deliveries (
			id, merchant_event_id, webhook_endpoint_id, status, attempt_count,
			next_attempt_at, created_at, updated_at
		)
		SELECT 'whd_' || merchant_event_id, merchant_event_id, webhook_endpoint_id,
			'pending', 0, $1, $1, $1
		FROM candidates
		ON CONFLICT (merchant_event_id) DO NOTHING
	`, now, limit)
	if err != nil {
		return 0, fmt.Errorf("materialize webhook deliveries: %w", err)
	}
	return result.RowsAffected(), nil
}

func (s *Store) ClaimDueDeliveries(ctx context.Context, now, lockedUntil time.Time, workerID string, limit int) ([]webhookdelivery.Job, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		WITH due AS (
			SELECT delivery.id
			FROM conver_pay.webhook_deliveries delivery
			JOIN conver_pay.merchant_webhook_endpoints endpoint ON endpoint.id=delivery.webhook_endpoint_id
			WHERE delivery.status IN ('pending','retry')
				AND delivery.next_attempt_at <= $1
				AND (delivery.locked_until IS NULL OR delivery.locked_until < $1)
				AND endpoint.enabled=true
			ORDER BY delivery.next_attempt_at, delivery.created_at, delivery.id
			FOR UPDATE OF delivery SKIP LOCKED
			LIMIT $4
		), claimed AS (
			UPDATE conver_pay.webhook_deliveries delivery
			SET locked_by=$2, locked_until=$3, updated_at=$1
			FROM due
			WHERE delivery.id=due.id
			RETURNING delivery.id, delivery.merchant_event_id, delivery.webhook_endpoint_id,
				delivery.attempt_count
		)
		SELECT claimed.id, claimed.merchant_event_id, event.event_type, event.payload,
			endpoint.url, endpoint.signing_secret_ciphertext, claimed.attempt_count
		FROM claimed
		JOIN conver_pay.merchant_events event ON event.id=claimed.merchant_event_id
		JOIN conver_pay.merchant_webhook_endpoints endpoint ON endpoint.id=claimed.webhook_endpoint_id
		ORDER BY claimed.id
	`, now, workerID, lockedUntil, limit)
	if err != nil {
		return nil, fmt.Errorf("claim webhook deliveries: %w", err)
	}
	defer rows.Close()

	var jobs []webhookdelivery.Job
	for rows.Next() {
		var job webhookdelivery.Job
		if err := rows.Scan(&job.DeliveryID, &job.MerchantEventID, &job.EventType,
			&job.Payload, &job.TargetURL, &job.SigningSecretCiphertext, &job.AttemptCount); err != nil {
			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook deliveries: %w", err)
	}
	return jobs, nil
}

func (s *Store) CompleteDelivery(ctx context.Context, workerID string, completion webhookdelivery.Completion) error {
	latencyMS := completion.Latency.Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin webhook completion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO conver_pay.webhook_delivery_attempts (
			id, delivery_id, sequence, started_at, completed_at,
			http_status, latency_ms, error_code
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, completion.AttemptID, completion.DeliveryID, completion.Sequence,
		completion.StartedAt, completion.CompletedAt, completion.HTTPStatus,
		latencyMS, completion.ErrorCode)
	if err != nil {
		return fmt.Errorf("insert webhook delivery attempt: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE conver_pay.webhook_deliveries SET
			status=$3,
			attempt_count=$4,
			next_attempt_at=$5,
			locked_by='',
			locked_until=NULL,
			last_attempt_at=$6,
			delivered_at=$7,
			last_http_status=$8,
			last_error_code=$9,
			updated_at=$6
		WHERE id=$1 AND locked_by=$2
	`, completion.DeliveryID, workerID, completion.Status, completion.Sequence,
		completion.NextAttemptAt, completion.CompletedAt, completion.DeliveredAt,
		completion.HTTPStatus, completion.ErrorCode)
	if err != nil {
		return fmt.Errorf("update webhook delivery: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit webhook completion: %w", err)
	}
	return nil
}

var _ webhookdelivery.Repository = (*Store)(nil)
