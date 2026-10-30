-- +goose Up
CREATE TABLE IF NOT EXISTS pastes (
    id UUID PRIMARY KEY,
    short_code VARCHAR(8) NOT NULL UNIQUE,
    user_id UUID,
    content TEXT NOT NULL,
    visibility TEXT NOT NULL
        CHECK (visibility IN ('public', 'private')),
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (expires_at > created_at)
);

-- +goose Down
DROP TABLE IF EXISTS pastes;