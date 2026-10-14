-- +goose Up
-- +goose StatementBegin
-- Claim-check metadata for response bodies stored in object storage (zstd).
-- body_ref may already exist from 00003 (re-added if a prior draft dropped it).
ALTER TABLE call_runs
    ADD COLUMN IF NOT EXISTS body_ref TEXT NULL,
    ADD COLUMN IF NOT EXISTS body_hash TEXT NULL,
    ADD COLUMN IF NOT EXISTS body_encoding TEXT NULL,
    ADD COLUMN IF NOT EXISTS response_keys JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE call_runs
    DROP CONSTRAINT IF EXISTS call_runs_body_encoding,
    ADD CONSTRAINT call_runs_body_encoding CHECK (
        body_encoding IS NULL OR body_encoding IN ('zstd')
    );

CREATE INDEX IF NOT EXISTS idx_call_runs_response_keys ON call_runs USING GIN (response_keys jsonb_path_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_call_runs_response_keys;
ALTER TABLE call_runs
    DROP CONSTRAINT IF EXISTS call_runs_body_encoding,
    DROP COLUMN IF EXISTS response_keys,
    DROP COLUMN IF EXISTS body_encoding,
    DROP COLUMN IF EXISTS body_hash;
-- +goose StatementEnd
