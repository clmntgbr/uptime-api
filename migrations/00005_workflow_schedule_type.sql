-- +goose Up
-- +goose StatementBegin
ALTER TABLE workflows
    ADD COLUMN schedule_type TEXT NOT NULL DEFAULT 'interval',
    ADD COLUMN run_at TIMESTAMPTZ NULL;

ALTER TABLE workflows
    ALTER COLUMN interval_seconds DROP NOT NULL;

ALTER TABLE workflows
    DROP CONSTRAINT IF EXISTS workflows_interval_min;

ALTER TABLE workflows
    ADD CONSTRAINT workflows_schedule_type CHECK (schedule_type IN ('interval', 'once', 'at')),
    ADD CONSTRAINT workflows_schedule_fields CHECK (
        (schedule_type = 'interval' AND interval_seconds IS NOT NULL AND interval_seconds >= 60 AND run_at IS NULL)
        OR (schedule_type = 'once' AND interval_seconds IS NULL AND run_at IS NULL)
        OR (schedule_type = 'at' AND interval_seconds IS NULL AND run_at IS NOT NULL)
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE workflows
SET interval_seconds = 300
WHERE interval_seconds IS NULL;

ALTER TABLE workflows
    DROP CONSTRAINT IF EXISTS workflows_schedule_fields,
    DROP CONSTRAINT IF EXISTS workflows_schedule_type;

ALTER TABLE workflows
    ALTER COLUMN interval_seconds SET NOT NULL;

ALTER TABLE workflows
    ADD CONSTRAINT workflows_interval_min CHECK (interval_seconds >= 60);

ALTER TABLE workflows
    DROP COLUMN IF EXISTS run_at,
    DROP COLUMN IF EXISTS schedule_type;
-- +goose StatementEnd
