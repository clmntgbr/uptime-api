-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL,
    timeout_ms INTEGER NOT NULL DEFAULT 300000,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workflows_interval_min CHECK (interval_seconds >= 60),
    CONSTRAINT workflows_timeout_range CHECK (timeout_ms BETWEEN 1000 AND 300000)
);

CREATE INDEX IF NOT EXISTS idx_workflows_user_id ON workflows (user_id);

CREATE TABLE IF NOT EXISTS calls (
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

CREATE TABLE IF NOT EXISTS rules (
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
CREATE TABLE IF NOT EXISTS variables (
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

CREATE UNIQUE INDEX IF NOT EXISTS uq_variables_global_name ON variables (user_id, name) WHERE type = 'global';
CREATE UNIQUE INDEX IF NOT EXISTS uq_variables_static_name ON variables (workflow_id, name) WHERE type = 'static';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS variables;
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS calls;
DROP TABLE IF EXISTS workflows;
-- +goose StatementEnd
