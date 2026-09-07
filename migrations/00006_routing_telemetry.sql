-- +goose Up
ALTER TABLE conver_pay.provider_connections
    DROP CONSTRAINT provider_connections_circuit_state_check;

ALTER TABLE conver_pay.provider_connections
    ADD CONSTRAINT provider_connections_circuit_state_check
    CHECK (circuit_state IN ('closed', 'open', 'half_open'));

ALTER TABLE conver_pay.provider_connections
    ADD COLUMN circuit_opened_at TIMESTAMPTZ;

UPDATE conver_pay.provider_connections
SET circuit_opened_at=now()
WHERE circuit_state='open';

CREATE TABLE conver_pay.provider_health_snapshots (
    id BIGSERIAL PRIMARY KEY,
    provider_connection_id TEXT NOT NULL REFERENCES conver_pay.provider_connections(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL CHECK (sample_count >= 0),
    qr_success_count INTEGER NOT NULL CHECK (qr_success_count >= 0),
    error_count INTEGER NOT NULL CHECK (error_count >= 0),
    timeout_count INTEGER NOT NULL CHECK (timeout_count >= 0),
    qr_success_rate DOUBLE PRECISION NOT NULL CHECK (qr_success_rate >= 0 AND qr_success_rate <= 1),
    error_rate DOUBLE PRECISION NOT NULL CHECK (error_rate >= 0 AND error_rate <= 1),
    timeout_rate DOUBLE PRECISION NOT NULL CHECK (timeout_rate >= 0 AND timeout_rate <= 1),
    latency_p50_ms DOUBLE PRECISION NOT NULL CHECK (latency_p50_ms >= 0),
    latency_p95_ms DOUBLE PRECISION NOT NULL CHECK (latency_p95_ms >= 0),
    health_score DOUBLE PRECISION NOT NULL CHECK (health_score >= 0 AND health_score <= 1),
    score_version TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX provider_health_snapshots_connection_observed_idx
    ON conver_pay.provider_health_snapshots (provider_connection_id, observed_at DESC);

ALTER TABLE conver_pay.routing_decisions
    ADD COLUMN score_version TEXT NOT NULL DEFAULT 'priority-v1';

-- +goose Down
ALTER TABLE conver_pay.routing_decisions DROP COLUMN IF EXISTS score_version;
DROP TABLE IF EXISTS conver_pay.provider_health_snapshots;
ALTER TABLE conver_pay.provider_connections DROP COLUMN IF EXISTS circuit_opened_at;
ALTER TABLE conver_pay.provider_connections
    DROP CONSTRAINT provider_connections_circuit_state_check;
ALTER TABLE conver_pay.provider_connections
    ADD CONSTRAINT provider_connections_circuit_state_check
    CHECK (circuit_state IN ('closed', 'open'));
