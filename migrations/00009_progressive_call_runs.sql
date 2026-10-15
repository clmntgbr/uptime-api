-- +goose Up
-- +goose StatementBegin
-- Runs can stay open while calls stream in; call_runs are upserted by position.
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_status;
ALTER TABLE runs
    ADD CONSTRAINT runs_status CHECK (status IN ('running', 'success', 'failed', 'skipped', 'error'));

ALTER TABLE runs ALTER COLUMN finished_at DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_call_runs_run_position ON call_runs (run_id, position);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_call_runs_run_position;

UPDATE runs SET finished_at = COALESCE(finished_at, started_at) WHERE finished_at IS NULL;
UPDATE runs SET status = 'error' WHERE status = 'running';

ALTER TABLE runs ALTER COLUMN finished_at SET NOT NULL;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_status;
ALTER TABLE runs
    ADD CONSTRAINT runs_status CHECK (status IN ('success', 'failed', 'skipped', 'error'));
-- +goose StatementEnd
