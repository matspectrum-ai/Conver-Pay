-- +goose Up
CREATE TABLE conver_pay.provider_events (
    id TEXT PRIMARY KEY,
    provider_connection_id TEXT NOT NULL REFERENCES conver_pay.provider_connections(id),
    external_event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    provider_payment_id TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    CONSTRAINT provider_events_external_unique UNIQUE (provider_connection_id, external_event_id)
);

CREATE INDEX provider_events_connection_received_idx
    ON conver_pay.provider_events (provider_connection_id, received_at DESC);

CREATE TABLE conver_pay.merchant_events (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    event_key TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    payment_intent_id TEXT NOT NULL REFERENCES conver_pay.payment_intents(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX merchant_events_workspace_created_idx
    ON conver_pay.merchant_events (workspace_id, created_at, id);

-- +goose Down
DROP TABLE IF EXISTS conver_pay.merchant_events;
DROP TABLE IF EXISTS conver_pay.provider_events;
