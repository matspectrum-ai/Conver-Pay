-- +goose Up
CREATE TABLE conver_pay.provider_connections (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    provider_key TEXT NOT NULL,
    environment TEXT NOT NULL CHECK (environment IN ('test', 'live')),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    credentials_valid BOOLEAN NOT NULL DEFAULT FALSE,
    circuit_state TEXT NOT NULL DEFAULT 'closed' CHECK (circuit_state IN ('closed', 'open')),
    priority INTEGER NOT NULL DEFAULT 100 CHECK (priority >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX provider_connections_workspace_idx
    ON conver_pay.provider_connections (workspace_id, environment, priority, id);

CREATE TABLE conver_pay.payment_intents (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    merchant_order_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL CHECK (char_length(currency) = 3),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    active_attempt_id TEXT NOT NULL DEFAULT '',
    presented_attempt_id TEXT NOT NULL DEFAULT '',
    pix_copy_paste TEXT NOT NULL DEFAULT '',
    pix_expires_at TIMESTAMPTZ,
    recovered BOOLEAN NOT NULL DEFAULT FALSE,
    recovered_amount BIGINT NOT NULL DEFAULT 0 CHECK (recovered_amount >= 0),
    failure_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    CONSTRAINT payment_intents_workspace_idempotency_unique UNIQUE (workspace_id, idempotency_key)
);

CREATE INDEX payment_intents_workspace_created_idx
    ON conver_pay.payment_intents (workspace_id, created_at DESC);

CREATE TABLE conver_pay.payment_attempts (
    id TEXT PRIMARY KEY,
    payment_intent_id TEXT NOT NULL REFERENCES conver_pay.payment_intents(id) ON DELETE CASCADE,
    provider_connection_id TEXT NOT NULL REFERENCES conver_pay.provider_connections(id),
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    status TEXT NOT NULL,
    provider_payment_id TEXT NOT NULL DEFAULT '',
    failure_code TEXT NOT NULL DEFAULT '',
    reconciliation_status TEXT NOT NULL DEFAULT '',
    pix_copy_paste TEXT NOT NULL DEFAULT '',
    pix_expires_at TIMESTAMPTZ,
    request_started_at TIMESTAMPTZ,
    response_received_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_attempts_payment_sequence_unique UNIQUE (payment_intent_id, sequence)
);

CREATE INDEX payment_attempts_payment_idx
    ON conver_pay.payment_attempts (payment_intent_id, sequence);

CREATE TABLE conver_pay.routing_decisions (
    id TEXT PRIMARY KEY,
    payment_intent_id TEXT NOT NULL REFERENCES conver_pay.payment_intents(id) ON DELETE CASCADE,
    attempt_id TEXT NOT NULL REFERENCES conver_pay.payment_attempts(id) ON DELETE CASCADE,
    selected_provider_connection_id TEXT NOT NULL REFERENCES conver_pay.provider_connections(id),
    candidate_snapshot JSONB NOT NULL,
    reason_codes JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX routing_decisions_payment_idx
    ON conver_pay.routing_decisions (payment_intent_id, created_at);

CREATE TABLE conver_pay.recovery_events (
    id TEXT PRIMARY KEY,
    payment_intent_id TEXT NOT NULL REFERENCES conver_pay.payment_intents(id) ON DELETE CASCADE,
    failed_attempt_id TEXT NOT NULL REFERENCES conver_pay.payment_attempts(id),
    successful_attempt_id TEXT NOT NULL REFERENCES conver_pay.payment_attempts(id),
    failure_reason TEXT NOT NULL DEFAULT '',
    amount BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT recovery_events_one_per_payment UNIQUE (payment_intent_id)
);

-- +goose Down
DROP TABLE IF EXISTS conver_pay.recovery_events;
DROP TABLE IF EXISTS conver_pay.routing_decisions;
DROP TABLE IF EXISTS conver_pay.payment_attempts;
DROP TABLE IF EXISTS conver_pay.payment_intents;
DROP TABLE IF EXISTS conver_pay.provider_connections;
