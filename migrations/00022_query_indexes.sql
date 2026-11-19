-- +goose Up
-- +goose StatementBegin
-- Plain CREATE INDEX (not CONCURRENTLY: goose runs migrations in a transaction).
-- Each takes a short write lock on its table; run off-peak on large databases.

-- LockDue: the "expired lease" branch of the scheduler query had no index and
-- ran as a full scan of workflow_state on every tick.
CREATE INDEX IF NOT EXISTS idx_workflow_state_active_lease
    ON workflow_state (lease_until)
    WHERE status IN ('running', 'cancelling');

-- Latest terminal run per workflow: skipped/overlap rows are no longer scanned.
CREATE INDEX IF NOT EXISTS idx_runs_workflow_terminal
    ON runs (workflow_id, started_at DESC)
    WHERE status IN ('success', 'failed', 'error');

-- Run detail: rule runs come back ordered by call_run_id, position.
CREATE INDEX IF NOT EXISTS idx_rule_runs_run_order
    ON rule_runs (run_id, call_run_id, position);

-- Library and workflow lists sorted by date (default and updated sorts).
CREATE INDEX IF NOT EXISTS library_items_user_created_idx
    ON library_items (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS library_items_user_updated_idx
    ON library_items (user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS workflows_user_updated_active_idx
    ON workflows (user_id, updated_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS workflows_user_updated_active_idx;
DROP INDEX IF EXISTS library_items_user_updated_idx;
DROP INDEX IF EXISTS library_items_user_created_idx;
DROP INDEX IF EXISTS idx_rule_runs_run_order;
DROP INDEX IF EXISTS idx_runs_workflow_terminal;
DROP INDEX IF EXISTS idx_workflow_state_active_lease;
-- +goose StatementEnd
