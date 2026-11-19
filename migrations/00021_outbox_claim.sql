-- +goose Up
-- +goose StatementBegin
-- Lease column: a relay instance claims a batch for a short time so that several
-- worker replicas never publish the same outbox row at the same moment.
ALTER TABLE outbox_events ADD COLUMN claimed_until TIMESTAMPTZ NULL;

-- The relay only ever reads unpublished rows ordered by created_at: index exactly that.
DROP INDEX IF EXISTS idx_outbox_events_published_at;
CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_outbox_events_unpublished;
CREATE INDEX idx_outbox_events_published_at ON outbox_events (published_at);
ALTER TABLE outbox_events DROP COLUMN IF EXISTS claimed_until;
-- +goose StatementEnd
