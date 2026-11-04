-- +goose Up
-- +goose StatementBegin
-- Workflow incidents: policies, signal counters, open/ack/resolved rows, timeline.

CREATE TABLE incident_policies (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    enabled BOOLEAN NOT NULL,
    severity TEXT NOT NULL,
    open_after INT NOT NULL DEFAULT 3,
    close_after INT NOT NULL DEFAULT 2,
    params JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT incident_policies_workflow_kind UNIQUE (workflow_id, kind),
    CONSTRAINT incident_policies_kind CHECK (kind IN (
        'call_failed', 'workflow_failed', 'call_slow', 'chain_broken',
        'availability_low', 'no_run', 'tls_expiry'
    )),
    CONSTRAINT incident_policies_severity CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT incident_policies_open_after CHECK (open_after BETWEEN 1 AND 20),
    CONSTRAINT incident_policies_close_after CHECK (close_after BETWEEN 1 AND 20)
);

CREATE TABLE signal_state (
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    consecutive_failures INT NOT NULL DEFAULT 0,
    consecutive_successes INT NOT NULL DEFAULT 0,
    last_run_id UUID,
    last_signal_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workflow_id, fingerprint)
);

CREATE TABLE incidents (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    call_id UUID,
    fingerprint TEXT NOT NULL,
    severity TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'open',
    cause JSONB NOT NULL DEFAULT '{}'::jsonb,
    opened_at TIMESTAMPTZ NOT NULL,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by TEXT,
    resolved_at TIMESTAMPTZ,
    reopen_count INT NOT NULL DEFAULT 0,
    opened_by_run_id UUID,
    last_failure_run_id UUID,
    last_reminder_at TIMESTAMPTZ,
    reminder_count INT NOT NULL DEFAULT 0,
    CONSTRAINT incidents_kind CHECK (kind IN (
        'call_failed', 'workflow_failed', 'call_slow', 'chain_broken',
        'availability_low', 'no_run', 'tls_expiry'
    )),
    CONSTRAINT incidents_severity CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT incidents_state CHECK (state IN ('open', 'acknowledged', 'resolved'))
);

CREATE UNIQUE INDEX incidents_one_active
    ON incidents (workflow_id, fingerprint)
    WHERE state <> 'resolved';

CREATE INDEX idx_incidents_user_state ON incidents (user_id, state, opened_at DESC);
CREATE INDEX idx_incidents_workflow_state ON incidents (workflow_id, state, opened_at DESC);

CREATE TABLE incident_events (
    id UUID PRIMARY KEY,
    incident_id UUID NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    at TIMESTAMPTZ NOT NULL,
    type TEXT NOT NULL,
    run_id UUID,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_incident_events_incident_at ON incident_events (incident_id, at);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON incident_policies, signal_state, incidents, incident_events
            TO uptime_api, uptime_cli;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_worker') THEN
        GRANT SELECT ON incident_policies TO uptime_worker;
        GRANT SELECT, INSERT, UPDATE ON signal_state, incidents, incident_events TO uptime_worker;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_scheduler') THEN
        GRANT SELECT ON incident_policies, workflows TO uptime_scheduler;
        GRANT SELECT, INSERT, UPDATE ON signal_state, incidents, incident_events TO uptime_scheduler;
        GRANT SELECT, INSERT ON outbox_events, processed_events TO uptime_scheduler;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS incident_events;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS signal_state;
DROP TABLE IF EXISTS incident_policies;
-- +goose StatementEnd
