-- +goose Up
-- +goose StatementBegin
ALTER TABLE calls
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE calls DROP COLUMN IF EXISTS description;
-- +goose StatementEnd
