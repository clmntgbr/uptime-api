-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workflow_state (
    workflow_id UUID PRIMARY KEY REFERENCES workflows (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'idle',
    next_run_at TIMESTAMPTZ NOT NULL,
    lease_run_id UUID NULL,
    lease_started_at TIMESTAMPTZ NULL,
    lease_until TIMESTAMPTZ NULL,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    consecutive_successes INTEGER NOT NULL DEFAULT 0,
    last_run_id UUID NULL,
    last_run_at TIMESTAMPTZ NULL,
    last_status TEXT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workflow_state_status CHECK (status IN ('idle', 'running')),
    CONSTRAINT workflow_state_lease CHECK (
        (status = 'idle' AND lease_run_id IS NULL)
        OR (status = 'running' AND lease_run_id IS NOT NULL AND lease_until IS NOT NULL AND lease_started_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_workflow_state_next_run_at ON workflow_state (next_run_at);

CREATE TABLE IF NOT EXISTS certificates (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    chain_expires_at TIMESTAMPTZ NOT NULL,
    chain_valid BOOLEAN NOT NULL,
    name_match BOOLEAN NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT certificates_user_host_port_key UNIQUE (user_id, host, port)
);

CREATE INDEX IF NOT EXISTS idx_certificates_user_expires ON certificates (user_id, chain_expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS certificates;
DROP TABLE IF EXISTS workflow_state;
-- +goose StatementEnd
