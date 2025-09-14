-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pending_signups (
    session_id VARCHAR(255) PRIMARY KEY,
    login VARCHAR(255) NOT NULL,
    salt_root BYTEA NOT NULL,
    kdf BYTEA NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    consumed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_signups_expires_at ON pending_signups (expires_at);
CREATE INDEX IF NOT EXISTS idx_pending_signups_login ON pending_signups (login);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pending_signups;
-- +goose StatementEnd
