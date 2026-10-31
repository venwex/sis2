-- +goose Up
CREATE TABLE paste_creation_keys (
    idempotency_key UUID PRIMARY KEY,
    request_hash TEXT NOT NULL,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE paste_creation_keys;