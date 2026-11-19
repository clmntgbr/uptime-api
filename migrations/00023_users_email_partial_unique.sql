-- +goose Up
-- +goose StatementBegin
-- Users without an email (phone-only Clerk accounts) store ''. A plain UNIQUE on
-- email makes the second such user fail with a unique violation, and that
-- failure surfaces as a permanent 503 at login. Uniqueness only applies to real addresses.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_nonempty ON users (email) WHERE email <> '';

-- Incident reminders scan open incidents only.
CREATE INDEX IF NOT EXISTS idx_incidents_open_opened_at ON incidents (opened_at) WHERE state = 'open';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_incidents_open_opened_at;
DROP INDEX IF EXISTS users_email_unique_nonempty;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
-- +goose StatementEnd
