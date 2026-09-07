package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type scanner interface {
	Scan(dest ...any) error
}

const paymentColumns = `
	id, workspace_id, merchant_order_id, idempotency_key, request_fingerprint,
	amount, currency, metadata, status, active_attempt_id, presented_attempt_id,
	pix_copy_paste, pix_expires_at, recovered, recovered_amount, failure_code,
	created_at, updated_at, paid_at`

func (s *Store) GetOrCreatePayment(ctx context.Context, intent *domain.PaymentIntent) (*domain.PaymentIntent, bool, error) {
	metadata, err := json.Marshal(intent.Metadata)
	if err != nil {
		return nil, false, fmt.Errorf("marshal payment metadata: %w", err)
	}
	pixCopy, pixExpires := pixValues(intent.Pix)

	row := s.pool.QueryRow(ctx, `
		INSERT INTO conver_pay.payment_intents (
			id, workspace_id, merchant_order_id, idempotency_key, request_fingerprint,
			amount, currency, metadata, status, active_attempt_id, presented_attempt_id,
			pix_copy_paste, pix_expires_at, recovered, recovered_amount, failure_code,
			created_at, updated_at, paid_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
		)
		ON CONFLICT (workspace_id, idempotency_key) DO NOTHING
		RETURNING `+paymentColumns,
		intent.ID, intent.WorkspaceID, intent.MerchantOrderID, intent.IdempotencyKey,
		intent.RequestFingerprint, intent.Amount, intent.Currency, metadata, intent.Status,
		intent.ActiveAttemptID, intent.PresentedAttemptID, pixCopy, pixExpires,
		intent.Recovered, intent.RecoveredAmount, intent.FailureCode,
		intent.CreatedAt, intent.UpdatedAt, intent.PaidAt,
	)
	inserted, err := scanPayment(row)
	if err == nil {
		return inserted, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("insert payment intent: %w", err)
	}

	existing, err := s.getPaymentByIdempotency(ctx, intent.WorkspaceID, intent.IdempotencyKey)
	if err != nil {
		return nil, false, err
	}
	if existing.RequestFingerprint != intent.RequestFingerprint {
		return nil, false, domain.ErrIdempotencyConflict
	}
	return existing, false, nil
}

func (s *Store) getPaymentByIdempotency(ctx context.Context, workspaceID, idempotencyKey string) (*domain.PaymentIntent, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+paymentColumns+`
		FROM conver_pay.payment_intents
		WHERE workspace_id = $1 AND idempotency_key = $2`, workspaceID, idempotencyKey)
	intent, err := scanPayment(row)
	return intent, mapNotFound(err)
}

func (s *Store) GetPayment(ctx context.Context, id string) (*domain.PaymentIntent, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+paymentColumns+`
		FROM conver_pay.payment_intents WHERE id = $1`, id)
	intent, err := scanPayment(row)
	return intent, mapNotFound(err)
}

func (s *Store) SavePayment(ctx context.Context, intent *domain.PaymentIntent) error {
	metadata, err := json.Marshal(intent.Metadata)
	if err != nil {
		return fmt.Errorf("marshal payment metadata: %w", err)
	}
	pixCopy, pixExpires := pixValues(intent.Pix)
	result, err := s.pool.Exec(ctx, `
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
		return fmt.Errorf("update payment intent: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListProviderConnections(ctx context.Context, workspaceID string) ([]domain.ProviderConnection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, provider_key, environment, enabled,
			credentials_valid, circuit_state, circuit_opened_at, priority
		FROM conver_pay.provider_connections
		WHERE workspace_id = $1
		ORDER BY priority, id`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list provider connections: %w", err)
	}
	defer rows.Close()

	var result []domain.ProviderConnection
	for rows.Next() {
		var conn domain.ProviderConnection
		if err := rows.Scan(&conn.ID, &conn.WorkspaceID, &conn.ProviderKey, &conn.Environment,
			&conn.Enabled, &conn.CredentialsValid, &conn.Circuit, &conn.CircuitOpenedAt, &conn.Priority); err != nil {
			return nil, fmt.Errorf("scan provider connection: %w", err)
		}
		result = append(result, conn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider connections: %w", err)
	}
	return result, nil
}

func (s *Store) AddAttemptWithRoutingDecision(ctx context.Context, attempt *domain.PaymentAttempt, decision *domain.RoutingDecision) error {
	candidates, err := json.Marshal(decision.Candidates)
	if err != nil {
		return fmt.Errorf("marshal routing candidates: %w", err)
	}
	reasons, err := json.Marshal(decision.ReasonCodes)
	if err != nil {
		return fmt.Errorf("marshal routing reason codes: %w", err)
	}
	pixCopy, pixExpires := pixValues(attempt.Pix)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin attempt transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO conver_pay.payment_attempts (
			id, payment_intent_id, provider_connection_id, sequence, status,
			provider_payment_id, failure_code, reconciliation_status,
			pix_copy_paste, pix_expires_at, request_started_at, response_received_at,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		attempt.ID, attempt.PaymentIntentID, attempt.ProviderConnectionID, attempt.Sequence,
		attempt.Status, attempt.ProviderPaymentID, attempt.FailureCode,
		attempt.ReconciliationStatus, pixCopy, pixExpires, nullableTime(attempt.RequestStartedAt),
		attempt.ResponseReceivedAt, attempt.CreatedAt, attempt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert payment attempt: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO conver_pay.routing_decisions (
			id, payment_intent_id, attempt_id, selected_provider_connection_id,
			candidate_snapshot, reason_codes, score_version, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		decision.ID, decision.PaymentIntentID, decision.AttemptID,
		decision.SelectedProviderConnectionID, candidates, reasons, decision.ScoreVersion, decision.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert routing decision: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit attempt transaction: %w", err)
	}
	return nil
}

func (s *Store) SaveAttempt(ctx context.Context, attempt *domain.PaymentAttempt) error {
	pixCopy, pixExpires := pixValues(attempt.Pix)
	result, err := s.pool.Exec(ctx, `
		UPDATE conver_pay.payment_attempts SET
			status=$2, provider_payment_id=$3, failure_code=$4, reconciliation_status=$5,
			pix_copy_paste=$6, pix_expires_at=$7, request_started_at=$8,
			response_received_at=$9, updated_at=$10
		WHERE id=$1`,
		attempt.ID, attempt.Status, attempt.ProviderPaymentID, attempt.FailureCode,
		attempt.ReconciliationStatus, pixCopy, pixExpires, nullableTime(attempt.RequestStartedAt),
		attempt.ResponseReceivedAt, attempt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update payment attempt: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const attemptColumns = `
	id, payment_intent_id, provider_connection_id, sequence, status,
	provider_payment_id, failure_code, reconciliation_status, pix_copy_paste,
	pix_expires_at, request_started_at, response_received_at, created_at, updated_at`

func (s *Store) GetAttempt(ctx context.Context, id string) (*domain.PaymentAttempt, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+attemptColumns+`
		FROM conver_pay.payment_attempts WHERE id=$1`, id)
	attempt, err := scanAttempt(row)
	return attempt, mapNotFound(err)
}

func (s *Store) ListAttempts(ctx context.Context, paymentID string) ([]domain.PaymentAttempt, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+attemptColumns+`
		FROM conver_pay.payment_attempts WHERE payment_intent_id=$1 ORDER BY sequence`, paymentID)
	if err != nil {
		return nil, fmt.Errorf("list payment attempts: %w", err)
	}
	defer rows.Close()

	var result []domain.PaymentAttempt
	for rows.Next() {
		attempt, err := scanAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment attempt: %w", err)
		}
		result = append(result, *attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payment attempts: %w", err)
	}
	return result, nil
}

func (s *Store) ListRoutingDecisions(ctx context.Context, paymentID string) ([]domain.RoutingDecision, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, payment_intent_id, attempt_id, selected_provider_connection_id,
			candidate_snapshot, reason_codes, score_version, created_at
		FROM conver_pay.routing_decisions
		WHERE payment_intent_id=$1 ORDER BY created_at, id`, paymentID)
	if err != nil {
		return nil, fmt.Errorf("list routing decisions: %w", err)
	}
	defer rows.Close()

	var result []domain.RoutingDecision
	for rows.Next() {
		var d domain.RoutingDecision
		var candidates, reasons []byte
		if err := rows.Scan(&d.ID, &d.PaymentIntentID, &d.AttemptID,
			&d.SelectedProviderConnectionID, &candidates, &reasons, &d.ScoreVersion, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan routing decision: %w", err)
		}
		if err := json.Unmarshal(candidates, &d.Candidates); err != nil {
			return nil, fmt.Errorf("decode routing candidates: %w", err)
		}
		if err := json.Unmarshal(reasons, &d.ReasonCodes); err != nil {
			return nil, fmt.Errorf("decode routing reason codes: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routing decisions: %w", err)
	}
	return result, nil
}

func (s *Store) CreateRecoveryIfAbsent(ctx context.Context, event *domain.RecoveryEvent) (bool, error) {
	result, err := s.pool.Exec(ctx, `
		INSERT INTO conver_pay.recovery_events (
			id, payment_intent_id, failed_attempt_id, successful_attempt_id,
			failure_reason, amount, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (payment_intent_id) DO NOTHING`,
		event.ID, event.PaymentIntentID, event.FailedAttemptID, event.SuccessfulAttemptID,
		event.FailureReason, event.Amount, event.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert recovery event: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *Store) ListRecoveryEvents(ctx context.Context, paymentID string) ([]domain.RecoveryEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, payment_intent_id, failed_attempt_id, successful_attempt_id,
			failure_reason, amount, created_at
		FROM conver_pay.recovery_events WHERE payment_intent_id=$1 ORDER BY created_at`, paymentID)
	if err != nil {
		return nil, fmt.Errorf("list recovery events: %w", err)
	}
	defer rows.Close()

	var result []domain.RecoveryEvent
	for rows.Next() {
		var event domain.RecoveryEvent
		if err := rows.Scan(&event.ID, &event.PaymentIntentID, &event.FailedAttemptID,
			&event.SuccessfulAttemptID, &event.FailureReason, &event.Amount, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recovery event: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recovery events: %w", err)
	}
	return result, nil
}

func scanPayment(row scanner) (*domain.PaymentIntent, error) {
	var intent domain.PaymentIntent
	var metadata []byte
	var pixCopy string
	var pixExpires *time.Time
	if err := row.Scan(
		&intent.ID, &intent.WorkspaceID, &intent.MerchantOrderID, &intent.IdempotencyKey,
		&intent.RequestFingerprint, &intent.Amount, &intent.Currency, &metadata, &intent.Status,
		&intent.ActiveAttemptID, &intent.PresentedAttemptID, &pixCopy, &pixExpires,
		&intent.Recovered, &intent.RecoveredAmount, &intent.FailureCode,
		&intent.CreatedAt, &intent.UpdatedAt, &intent.PaidAt,
	); err != nil {
		return nil, err
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &intent.Metadata); err != nil {
			return nil, fmt.Errorf("decode payment metadata: %w", err)
		}
	}
	if pixCopy != "" {
		intent.Pix = &domain.Pix{CopyPaste: pixCopy}
		if pixExpires != nil {
			intent.Pix.ExpiresAt = *pixExpires
		}
	}
	return &intent, nil
}

func scanAttempt(row scanner) (*domain.PaymentAttempt, error) {
	var attempt domain.PaymentAttempt
	var pixCopy string
	var pixExpires, requestStarted *time.Time
	if err := row.Scan(
		&attempt.ID, &attempt.PaymentIntentID, &attempt.ProviderConnectionID,
		&attempt.Sequence, &attempt.Status, &attempt.ProviderPaymentID,
		&attempt.FailureCode, &attempt.ReconciliationStatus, &pixCopy, &pixExpires,
		&requestStarted, &attempt.ResponseReceivedAt, &attempt.CreatedAt, &attempt.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if requestStarted != nil {
		attempt.RequestStartedAt = *requestStarted
	}
	if pixCopy != "" {
		attempt.Pix = &domain.Pix{CopyPaste: pixCopy}
		if pixExpires != nil {
			attempt.Pix.ExpiresAt = *pixExpires
		}
	}
	return &attempt, nil
}

func pixValues(pix *domain.Pix) (string, *time.Time) {
	if pix == nil {
		return "", nil
	}
	expires := pix.ExpiresAt
	return pix.CopyPaste, &expires
}

func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
