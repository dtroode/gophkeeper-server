-- +goose Up
-- +goose StatementBegin

-- Создаем enum тип для типов записей
CREATE TYPE record_type AS ENUM ('login', 'note', 'card', 'binary');

CREATE TABLE IF NOT EXISTS records (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    encrypted_data BYTEA,
    s3_key VARCHAR(1024),
    encrypted_chunk_size INT DEFAULT 0,
    encrypted_key BYTEA NOT NULL,
    alg VARCHAR(50) NOT NULL,
    type record_type NOT NULL,
    request_id UUID NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL,

    CONSTRAINT fk_records_owner_id FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Индексы для производительности
CREATE INDEX IF NOT EXISTS idx_records_owner_id ON records (owner_id);
CREATE INDEX IF NOT EXISTS idx_records_type ON records (type);
CREATE INDEX IF NOT EXISTS idx_records_owner_type ON records (owner_id, type);
CREATE INDEX IF NOT EXISTS idx_records_s3_key ON records (s3_key);
CREATE INDEX IF NOT EXISTS idx_records_created_at ON records (created_at);
CREATE INDEX IF NOT EXISTS idx_records_deleted_at ON records (deleted_at) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uid_records_owner_request ON records (owner_id, request_id) WHERE request_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS records;
DROP TYPE IF EXISTS record_type;
-- +goose StatementEnd
