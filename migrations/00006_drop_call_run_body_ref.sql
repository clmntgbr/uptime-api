-- +goose Up
-- +goose StatementBegin
ALTER TABLE call_runs DROP COLUMN IF EXISTS body_ref;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE call_runs ADD COLUMN IF NOT EXISTS body_ref TEXT NULL;
-- +goose StatementEnd
