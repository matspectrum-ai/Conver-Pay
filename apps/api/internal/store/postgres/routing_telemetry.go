package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
	"github.com/matspectrum-ai/conver-pay/apps/api/internal/routingtelemetry"
)

func (s *Store) AggregateProviderHealth(ctx context.Context, connectionID string, windowStart, windowEnd time.Time) (domain.ProviderHealthSnapshot, error) {
	var snapshot domain.ProviderHealthSnapshot
	var successCount, errorCount, timeoutCount int
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*)::int,
			count(*) FILTER (WHERE status='succeeded')::int,
			count(*) FILTER (WHERE status IN ('failed_safe','failed_terminal'))::int,
			count(*) FILTER (WHERE status='unknown')::int,
			COALESCE(percentile_cont(0.50) WITHIN GROUP (
				ORDER BY EXTRACT(EPOCH FROM (response_received_at-request_started_at))*1000
			) FILTER (WHERE request_started_at IS NOT NULL AND response_received_at IS NOT NULL), 0)::double precision,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (
				ORDER BY EXTRACT(EPOCH FROM (response_received_at-request_started_at))*1000
			) FILTER (WHERE request_started_at IS NOT NULL AND response_received_at IS NOT NULL), 0)::double precision
		FROM conver_pay.payment_attempts
		WHERE provider_connection_id=$1
			AND request_started_at >= $2
			AND request_started_at < $3
	`, connectionID, windowStart, windowEnd).Scan(
		&snapshot.SampleCount, &successCount, &errorCount, &timeoutCount,
		&snapshot.LatencyP50MS, &snapshot.LatencyP95MS,
	)
	if err != nil {
		return domain.ProviderHealthSnapshot{}, fmt.Errorf("aggregate provider health: %w", err)
	}
	snapshot.ProviderConnectionID = connectionID
	snapshot.QRSuccessCount = successCount
	snapshot.ErrorCount = errorCount
	snapshot.TimeoutCount = timeoutCount
	if snapshot.SampleCount > 0 {
		denominator := float64(snapshot.SampleCount)
		snapshot.QRSuccessRate = float64(successCount) / denominator
		snapshot.ErrorRate = float64(errorCount) / denominator
		snapshot.TimeoutRate = float64(timeoutCount) / denominator
	}
	return snapshot, nil
}

func (s *Store) SaveProviderHealthSnapshot(ctx context.Context, snapshot *domain.ProviderHealthSnapshot) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO conver_pay.provider_health_snapshots (
			provider_connection_id, window_start, window_end, sample_count,
			qr_success_count, error_count, timeout_count, qr_success_rate,
			error_rate, timeout_rate, latency_p50_ms, latency_p95_ms,
			health_score, score_version, observed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, snapshot.ProviderConnectionID, snapshot.WindowStart, snapshot.WindowEnd,
		snapshot.SampleCount, snapshot.QRSuccessCount, snapshot.ErrorCount,
		snapshot.TimeoutCount, snapshot.QRSuccessRate, snapshot.ErrorRate,
		snapshot.TimeoutRate, snapshot.LatencyP50MS, snapshot.LatencyP95MS,
		snapshot.HealthScore, snapshot.ScoreVersion, snapshot.ObservedAt)
	if err != nil {
		return fmt.Errorf("save provider health snapshot: %w", err)
	}
	return nil
}

func (s *Store) ListRecentProviderHealthSnapshots(ctx context.Context, connectionID string, limit int) ([]domain.ProviderHealthSnapshot, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT provider_connection_id, window_start, window_end, sample_count,
			qr_success_count, error_count, timeout_count, qr_success_rate,
			error_rate, timeout_rate, latency_p50_ms, latency_p95_ms,
			health_score, score_version, observed_at
		FROM conver_pay.provider_health_snapshots
		WHERE provider_connection_id=$1
		ORDER BY observed_at DESC, id DESC
		LIMIT $2
	`, connectionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list provider health snapshots: %w", err)
	}
	defer rows.Close()
	var result []domain.ProviderHealthSnapshot
	for rows.Next() {
		var snapshot domain.ProviderHealthSnapshot
		if err := rows.Scan(
			&snapshot.ProviderConnectionID, &snapshot.WindowStart, &snapshot.WindowEnd,
			&snapshot.SampleCount, &snapshot.QRSuccessCount, &snapshot.ErrorCount,
			&snapshot.TimeoutCount, &snapshot.QRSuccessRate, &snapshot.ErrorRate,
			&snapshot.TimeoutRate, &snapshot.LatencyP50MS, &snapshot.LatencyP95MS,
			&snapshot.HealthScore, &snapshot.ScoreVersion, &snapshot.ObservedAt,
		); err != nil {
			return nil, fmt.Errorf("scan provider health snapshot: %w", err)
		}
		result = append(result, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider health snapshots: %w", err)
	}
	return result, nil
}

func (s *Store) LatestProviderAttemptSince(ctx context.Context, connectionID string, since time.Time) (*domain.PaymentAttempt, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+attemptColumns+`
		FROM conver_pay.payment_attempts
		WHERE provider_connection_id=$1 AND request_started_at >= $2
		ORDER BY request_started_at DESC, id DESC
		LIMIT 1`, connectionID, since)
	attempt, err := scanAttempt(row)
	return attempt, mapNotFound(err)
}

func (s *Store) UpdateProviderCircuit(ctx context.Context, connectionID string, state domain.CircuitState, openedAt *time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE conver_pay.provider_connections
		SET circuit_state=$2, circuit_opened_at=$3, updated_at=now()
		WHERE id=$1
	`, connectionID, state, openedAt)
	if err != nil {
		return fmt.Errorf("update provider circuit: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ routingtelemetry.Repository = (*Store)(nil)
