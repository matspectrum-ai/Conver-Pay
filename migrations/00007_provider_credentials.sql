-- +goose Up
CREATE TABLE conver_pay.provider_credentials (
    provider_connection_id TEXT PRIMARY KEY REFERENCES conver_pay.provider_connections(id) ON DELETE CASCADE,
    ciphertext TEXT NOT NULL,
    validated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS conver_pay.provider_credentials;
