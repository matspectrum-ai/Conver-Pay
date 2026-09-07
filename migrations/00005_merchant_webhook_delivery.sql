-- +goose Up
CREATE TABLE conver_pay.merchant_webhook_endpoints (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    environment TEXT NOT NULL CHECK (environment IN ('test', 'live')),
    url TEXT NOT NULL,
    signing_secret_ciphertext TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT merchant_webhook_endpoints_scope_unique UNIQUE (workspace_id, environment)
);

CREATE TABLE conver_pay.webhook_deliveries (
    id TEXT PRIMARY KEY,
    merchant_event_id TEXT NOT NULL REFERENCES conver_pay.merchant_events(id) ON DELETE CASCADE,
    webhook_endpoint_id TEXT NOT NULL REFERENCES conver_pay.merchant_webhook_endpoints(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'retry', 'succeeded', 'failed')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL,
    locked_by TEXT NOT NULL DEFAULT '',
    locked_until TIMESTAMPTZ,
    last_attempt_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    last_http_status INTEGER,
    last_error_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT webhook_deliveries_event_unique UNIQUE (merchant_event_id)
);

CREATE INDEX webhook_deliveries_due_idx
    ON conver_pay.webhook_deliveries (next_attempt_at, created_at, id)
    WHERE status IN ('pending', 'retry');

CREATE TABLE conver_pay.webhook_delivery_attempts (
    id TEXT PRIMARY KEY,
    delivery_id TEXT NOT NULL REFERENCES conver_pay.webhook_deliveries(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    http_status INTEGER,
    latency_ms BIGINT NOT NULL CHECK (latency_ms >= 0),
    error_code TEXT NOT NULL DEFAULT '',
    CONSTRAINT webhook_delivery_attempts_sequence_unique UNIQUE (delivery_id, sequence)
);

CREATE INDEX webhook_delivery_attempts_delivery_idx
    ON conver_pay.webhook_delivery_attempts (delivery_id, sequence);

-- +goose Down
DROP TABLE IF EXISTS conver_pay.webhook_delivery_attempts;
DROP TABLE IF EXISTS conver_pay.webhook_deliveries;
DROP TABLE IF EXISTS conver_pay.merchant_webhook_endpoints;
