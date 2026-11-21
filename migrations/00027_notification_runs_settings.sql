-- +goose Up
-- +goose StatementBegin
-- Re-introduce run notification family (dropped in 00017; needed for run.failed / run.errored mail).

ALTER TABLE workflow_notification_settings
    ADD COLUMN IF NOT EXISTS runs_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS run_statuses TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS run_mode TEXT NOT NULL DEFAULT 'transition';

ALTER TABLE workflow_notification_settings
    DROP CONSTRAINT IF EXISTS workflow_notification_settings_run_mode;
ALTER TABLE workflow_notification_settings
    ADD CONSTRAINT workflow_notification_settings_run_mode
        CHECK (run_mode IN ('every', 'transition'));

-- Default watched statuses when enabling runs later (empty = all non-success in app layer).
UPDATE workflow_notification_settings
SET run_statuses = ARRAY['failed', 'error']
WHERE cardinality(run_statuses) = 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workflow_notification_settings
    DROP CONSTRAINT IF EXISTS workflow_notification_settings_run_mode;
ALTER TABLE workflow_notification_settings
    DROP COLUMN IF EXISTS runs_enabled,
    DROP COLUMN IF EXISTS run_statuses,
    DROP COLUMN IF EXISTS run_mode;
-- +goose StatementEnd
