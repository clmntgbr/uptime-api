-- +goose Up
-- +goose StatementBegin
-- Chronological anomaly / threshold feed (append-only). workflow_id SET NULL keeps history when a workflow is deleted.

CREATE TABLE feed_events (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workflow_id UUID REFERENCES workflows (id) ON DELETE SET NULL,
    run_id UUID,
    call_id UUID,
    incident_id UUID,
    kind TEXT NOT NULL,
    severity TEXT NOT NULL,
    at TIMESTAMPTZ NOT NULL,
    title TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT feed_events_severity CHECK (severity IN ('info', 'success', 'warning', 'critical'))
);

CREATE INDEX idx_feed_events_user_at ON feed_events (user_id, at DESC, id DESC);
CREATE INDEX idx_feed_events_workflow_at ON feed_events (workflow_id, at DESC, id DESC);
CREATE INDEX idx_feed_events_incident ON feed_events (incident_id);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON feed_events TO uptime_api, uptime_cli;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_worker') THEN
        GRANT SELECT, INSERT ON feed_events TO uptime_worker;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_scheduler') THEN
        GRANT SELECT, INSERT, DELETE ON feed_events TO uptime_scheduler;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS feed_events;
-- +goose StatementEnd
