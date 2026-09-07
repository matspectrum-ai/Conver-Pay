-- +goose Up
CREATE TABLE conver_pay.api_keys (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    environment TEXT NOT NULL CHECK (environment IN ('test', 'live')),
    key_prefix TEXT NOT NULL,
    secret_hash BYTEA NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX api_keys_workspace_idx
    ON conver_pay.api_keys (workspace_id, environment, status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS conver_pay.api_keys;
