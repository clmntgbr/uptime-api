-- +goose Up
-- +goose StatementBegin
ALTER TABLE workflow_state
    DROP CONSTRAINT IF EXISTS workflow_state_status,
    DROP CONSTRAINT IF EXISTS workflow_state_lease;

ALTER TABLE workflow_state
    ADD CONSTRAINT workflow_state_status CHECK (status IN ('idle', 'running', 'cancelling')),
    ADD CONSTRAINT workflow_state_lease CHECK (
        (status = 'idle' AND lease_run_id IS NULL)
        OR (
            status IN ('running', 'cancelling')
            AND lease_run_id IS NOT NULL
            AND lease_until IS NOT NULL
            AND lease_started_at IS NOT NULL
        )
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE workflow_state SET status = 'running' WHERE status = 'cancelling';

ALTER TABLE workflow_state
    DROP CONSTRAINT IF EXISTS workflow_state_status,
    DROP CONSTRAINT IF EXISTS workflow_state_lease;

ALTER TABLE workflow_state
    ADD CONSTRAINT workflow_state_status CHECK (status IN ('idle', 'running')),
    ADD CONSTRAINT workflow_state_lease CHECK (
        (status = 'idle' AND lease_run_id IS NULL)
        OR (
            status = 'running'
            AND lease_run_id IS NOT NULL
            AND lease_until IS NOT NULL
            AND lease_started_at IS NOT NULL
        )
    );
-- +goose StatementEnd
