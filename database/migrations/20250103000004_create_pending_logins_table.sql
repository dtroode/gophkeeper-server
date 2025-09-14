-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pending_logins (
    session_id VARCHAR(255) PRIMARY KEY,
    login VARCHAR(255) NOT NULL,
    client_nonce BYTEA NOT NULL,
    server_nonce BYTEA NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    consumed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_logins_expires_at ON pending_logins (expires_at);
CREATE INDEX IF NOT EXISTS idx_pending_logins_login ON pending_logins (login);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pending_logins;
-- +goose StatementEnd

