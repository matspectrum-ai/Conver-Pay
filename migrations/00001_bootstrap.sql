-- +goose Up
CREATE SCHEMA IF NOT EXISTS conver_pay;
REVOKE ALL ON SCHEMA conver_pay FROM PUBLIC;
COMMENT ON SCHEMA conver_pay IS 'Private application schema for Conver Pay backend-owned data';

-- +goose Down
DROP SCHEMA IF EXISTS conver_pay CASCADE;
