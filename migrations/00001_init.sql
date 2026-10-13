-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_id TEXT NOT NULL,
    first_name TEXT NULL,
    last_name TEXT NULL,
    banned BOOLEAN NOT NULL DEFAULT FALSE,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_clerk_id_key UNIQUE (clerk_id),
    CONSTRAINT users_email_key UNIQUE (email)
);

CREATE INDEX idx_user_banned ON users (banned);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_outbox_events_aggregate_id ON outbox_events (aggregate_id);
CREATE INDEX idx_outbox_events_event_type ON outbox_events (event_type);
CREATE INDEX idx_outbox_events_published_at ON outbox_events (published_at);

-- Dedup key of the event handlers: (event_id, handler_name).
CREATE TABLE processed_events (
    event_id UUID NOT NULL,
    handler_name TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (event_id, handler_name)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS processed_events;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
