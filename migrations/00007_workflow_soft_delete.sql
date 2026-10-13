-- +goose Up
-- +goose StatementBegin
ALTER TABLE workflows ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

DROP INDEX IF EXISTS idx_workflows_user_id;
CREATE INDEX IF NOT EXISTS idx_workflows_user_active ON workflows (user_id, created_at) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_workflows_user_active;
CREATE INDEX IF NOT EXISTS idx_workflows_user_id ON workflows (user_id);
ALTER TABLE workflows DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
