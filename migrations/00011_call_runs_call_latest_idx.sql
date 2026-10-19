-- +goose Up
-- Latest call_run lookup per call (GET /workflows/:id/calls LATERAL).
CREATE INDEX IF NOT EXISTS idx_call_runs_call_latest
    ON call_runs (call_id, finished_at DESC NULLS LAST, started_at DESC NULLS LAST);

-- +goose Down
DROP INDEX IF EXISTS idx_call_runs_call_latest;
