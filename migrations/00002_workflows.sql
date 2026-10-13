-- +goose Up
-- +goose StatementBegin
-- Deleting a workflow is a soft delete (deleted_at): its calls, variables and
-- runs are kept, and every read filters on deleted_at IS NULL.
CREATE TABLE workflows (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL,
    timeout_ms INTEGER NOT NULL DEFAULT 300000,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT workflows_interval_min CHECK (interval_seconds >= 60),
    CONSTRAINT workflows_timeout_range CHECK (timeout_ms BETWEEN 1000 AND 300000)
);

CREATE INDEX idx_workflows_user_active ON workflows (user_id, created_at) WHERE deleted_at IS NULL;

CREATE TABLE calls (
    id UUID PRIMARY KEY,
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    name TEXT NOT NULL,
    method TEXT NOT NULL,
    url TEXT NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    query JSONB NOT NULL DEFAULT '{}'::jsonb,
    body TEXT NULL,
    timeout_ms INTEGER NOT NULL DEFAULT 30000,
    verify_tls BOOLEAN NOT NULL DEFAULT TRUE,
    ip_version TEXT NOT NULL DEFAULT 'any',
    proxy_url TEXT NULL,
    extractions JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT calls_workflow_position_key UNIQUE (workflow_id, position),
    CONSTRAINT calls_workflow_name_key UNIQUE (workflow_id, name),
    CONSTRAINT calls_timeout_range CHECK (timeout_ms BETWEEN 100 AND 60000),
    CONSTRAINT calls_ip_version CHECK (ip_version IN ('any', 'ipv4', 'ipv6'))
);

CREATE TABLE rules (
    id UUID PRIMARY KEY,
    call_id UUID NOT NULL REFERENCES calls (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    subject TEXT NOT NULL,
    operator TEXT NOT NULL,
    target TEXT NULL,
    value TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT rules_call_position_key UNIQUE (call_id, position)
);

-- `call` variables are not stored here: they are declared by calls.extractions.
CREATE TABLE variables (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workflow_id UUID NULL REFERENCES workflows (id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    value TEXT NOT NULL,
    secret BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT variables_type CHECK (type IN ('global', 'static')),
    CONSTRAINT variables_scope CHECK (
        (type = 'global' AND workflow_id IS NULL)
        OR (type = 'static' AND workflow_id IS NOT NULL AND secret = FALSE)
    )
);

CREATE UNIQUE INDEX uq_variables_global_name ON variables (user_id, name) WHERE type = 'global';
CREATE UNIQUE INDEX uq_variables_static_name ON variables (workflow_id, name) WHERE type = 'static';

-- Scheduling state, written by the scheduler and the worker, kept apart from
-- workflows so that editing a workflow and scheduling it do not contend.
CREATE TABLE workflow_state (
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

CREATE INDEX idx_workflow_state_next_run_at ON workflow_state (next_run_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workflow_state;
DROP TABLE IF EXISTS variables;
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS calls;
DROP TABLE IF EXISTS workflows;
-- +goose StatementEnd
