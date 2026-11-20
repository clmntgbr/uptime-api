-- +goose Up
-- +goose StatementBegin
-- The daily expiry pass reads items by expires_at range (see expiry.ScanWindow).
CREATE INDEX IF NOT EXISTS idx_expiry_items_expires_at ON expiry_items (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_expiry_items_expires_at;
-- +goose StatementEnd
