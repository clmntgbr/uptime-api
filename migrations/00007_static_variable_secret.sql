-- +goose Up
-- +goose StatementBegin
-- Workflow statics may be secret (encrypted at rest), like globals.
ALTER TABLE variables DROP CONSTRAINT IF EXISTS variables_scope;
ALTER TABLE variables
    ADD CONSTRAINT variables_scope CHECK (
        (type = 'global' AND workflow_id IS NULL)
        OR (type = 'static' AND workflow_id IS NOT NULL)
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE variables SET secret = FALSE WHERE type = 'static' AND secret = TRUE;
ALTER TABLE variables DROP CONSTRAINT IF EXISTS variables_scope;
ALTER TABLE variables
    ADD CONSTRAINT variables_scope CHECK (
        (type = 'global' AND workflow_id IS NULL)
        OR (type = 'static' AND workflow_id IS NOT NULL AND secret = FALSE)
    );
-- +goose StatementEnd
