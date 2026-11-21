-- +goose Up
-- +goose StatementBegin
ALTER TABLE workflow_notification_settings
    ADD COLUMN IF NOT EXISTS digest_daily BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS digest_weekly BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS digest_monthly BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workflow_notification_settings
    DROP COLUMN IF EXISTS digest_daily,
    DROP COLUMN IF EXISTS digest_weekly,
    DROP COLUMN IF EXISTS digest_monthly;
-- +goose StatementEnd
