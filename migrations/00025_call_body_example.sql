-- +goose Up
-- +goose StatementBegin
-- Reference payload kept beside the editable body (OpenAPI example, library template).
ALTER TABLE calls ADD COLUMN body_example TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE calls DROP COLUMN IF EXISTS body_example;
-- +goose StatementEnd
