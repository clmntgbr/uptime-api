-- +goose Up
-- +goose StatementBegin
ALTER TABLE workflows
    ADD COLUMN IF NOT EXISTS enabled_at TIMESTAMPTZ;

-- Backfill: treat last update as the monitoring arm time for already-enabled workflows.
UPDATE workflows
SET enabled_at = COALESCE(updated_at, created_at)
WHERE enabled_at IS NULL;

ALTER TABLE workflows
    ALTER COLUMN enabled_at SET NOT NULL,
    ALTER COLUMN enabled_at SET DEFAULT NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workflows DROP COLUMN IF EXISTS enabled_at;
-- +goose StatementEnd
