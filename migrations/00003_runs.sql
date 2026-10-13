-- +goose Up
-- +goose StatementBegin
-- Written by the worker (and the scheduler for skipped/timeout runs). The
-- retention deletes old runs; call and rule runs follow by foreign key. Run
-- idempotence is enforced through processed_events (event_id = run id).
CREATE TABLE runs (
    id UUID PRIMARY KEY,
    workflow_id UUID NOT NULL,
    user_id UUID NOT NULL,
    trigger TEXT NOT NULL,
    status TEXT NOT NULL,
    reason TEXT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    calls_total INTEGER NOT NULL DEFAULT 0,
    calls_success INTEGER NOT NULL DEFAULT 0,
    calls_failed INTEGER NOT NULL DEFAULT 0,
    calls_error INTEGER NOT NULL DEFAULT 0,
    calls_skipped INTEGER NOT NULL DEFAULT 0,
    variables JSONB NOT NULL DEFAULT '{}'::jsonb,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT runs_status CHECK (status IN ('success', 'failed', 'skipped', 'error')),
    CONSTRAINT runs_trigger CHECK (trigger IN ('schedule', 'manual', 'test'))
);

CREATE INDEX idx_runs_workflow_started ON runs (workflow_id, started_at DESC);
CREATE INDEX idx_runs_user_started ON runs (user_id, started_at DESC);
CREATE INDEX idx_runs_started ON runs (started_at);

CREATE TABLE call_runs (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL,
    call_id UUID NULL,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    status TEXT NOT NULL,
    reason TEXT NULL,
    http_status INTEGER NULL,
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    timings JSONB NOT NULL DEFAULT '{}'::jsonb,
    network JSONB NOT NULL DEFAULT '{}'::jsonb,
    tls JSONB NULL,
    extracted_variables JSONB NOT NULL DEFAULT '{}'::jsonb,
    used_variables JSONB NOT NULL DEFAULT '[]'::jsonb,
    error_type TEXT NULL,
    error_message TEXT NULL,
    body_ref TEXT NULL,
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT call_runs_status CHECK (status IN ('success', 'failed', 'error', 'skipped'))
);

CREATE INDEX idx_call_runs_run ON call_runs (run_id, position);

CREATE TABLE rule_runs (
    id UUID PRIMARY KEY,
    call_run_id UUID NOT NULL REFERENCES call_runs (id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    subject TEXT NOT NULL,
    operator TEXT NOT NULL,
    text TEXT NOT NULL,
    observed TEXT NOT NULL,
    expected TEXT NOT NULL,
    passed BOOLEAN NOT NULL
);

CREATE INDEX idx_rule_runs_call_run ON rule_runs (call_run_id, position);
CREATE INDEX idx_rule_runs_run ON rule_runs (run_id);

CREATE TABLE certificates (
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

CREATE INDEX idx_certificates_user_expires ON certificates (user_id, chain_expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS certificates;
DROP TABLE IF EXISTS rule_runs;
DROP TABLE IF EXISTS call_runs;
DROP TABLE IF EXISTS runs;
-- +goose StatementEnd
