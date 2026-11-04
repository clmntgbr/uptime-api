-- +goose Up
-- +goose StatementBegin
CREATE TABLE workflow_notification_settings (
    workflow_id UUID PRIMARY KEY REFERENCES workflows (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    emails TEXT[] NOT NULL DEFAULT '{}',
    incidents_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    tls_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    runs_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    run_statuses TEXT[] NOT NULL DEFAULT '{}',
    run_mode TEXT NOT NULL DEFAULT 'transition',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workflow_notification_settings_run_mode CHECK (run_mode IN ('every', 'transition'))
);

-- Backfill existing workflows with creator email (incidents+TLS on, runs off).
INSERT INTO workflow_notification_settings (
    workflow_id, user_id, emails,
    incidents_enabled, tls_enabled, runs_enabled, run_statuses, run_mode, updated_at
)
SELECT
    w.id,
    w.user_id,
    CASE
        WHEN COALESCE(NULLIF(trim(u.email), ''), '') = '' THEN '{}'::text[]
        ELSE ARRAY[lower(trim(u.email))]
    END,
    TRUE, TRUE, FALSE, '{}'::text[], 'transition', NOW()
FROM workflows w
JOIN users u ON u.id = w.user_id
WHERE w.deleted_at IS NULL
ON CONFLICT (workflow_id) DO NOTHING;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON workflow_notification_settings
            TO uptime_api, uptime_cli;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_worker') THEN
        GRANT SELECT ON workflow_notification_settings TO uptime_worker;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_scheduler') THEN
        GRANT SELECT ON workflow_notification_settings TO uptime_scheduler;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workflow_notification_settings;
-- +goose StatementEnd
