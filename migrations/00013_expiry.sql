-- +goose Up
-- +goose StatementBegin
-- Generic expiry inventory (phase 1: tls_certificate). Policies are optional:
-- missing rows mean defaults (enabled, thresholds 30/14/7/1).

CREATE TABLE expiry_policies (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    thresholds_days JSONB NOT NULL DEFAULT '[30,14,7,1]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT expiry_policies_workflow_type UNIQUE (workflow_id, type),
    CONSTRAINT expiry_policies_type CHECK (type IN ('tls_certificate')),
    CONSTRAINT expiry_policies_thresholds_len CHECK (jsonb_typeof(thresholds_days) = 'array' AND jsonb_array_length(thresholds_days) BETWEEN 1 AND 20)
);

CREATE TABLE expiry_items (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    subject TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    fingerprint TEXT NOT NULL,
    meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT expiry_items_workflow_type_subject UNIQUE (workflow_id, type, subject),
    CONSTRAINT expiry_items_type CHECK (type IN ('tls_certificate'))
);

CREATE INDEX idx_expiry_items_user_expires ON expiry_items (user_id, expires_at);
CREATE INDEX idx_expiry_items_observed ON expiry_items (observed_at);

CREATE TABLE expiry_alerts (
    item_id UUID NOT NULL REFERENCES expiry_items (id) ON DELETE CASCADE,
    threshold_days INT NOT NULL,
    fingerprint TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (item_id, threshold_days, fingerprint)
);

-- Certificate alerts move to expiry policies; drop call-level certificate rules.
DELETE FROM rules WHERE subject = 'certificate';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON expiry_policies TO uptime_api, uptime_cli;
        GRANT SELECT ON expiry_items, expiry_alerts TO uptime_api, uptime_cli;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_worker') THEN
        GRANT SELECT ON expiry_policies TO uptime_worker;
        GRANT SELECT, INSERT, UPDATE ON expiry_items, expiry_alerts TO uptime_worker;
        GRANT SELECT ON workflows, calls TO uptime_worker;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_scheduler') THEN
        GRANT SELECT ON expiry_policies, expiry_items TO uptime_scheduler;
        GRANT SELECT, INSERT ON expiry_alerts TO uptime_scheduler;
        GRANT SELECT, INSERT ON outbox_events, processed_events TO uptime_scheduler;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS expiry_alerts;
DROP TABLE IF EXISTS expiry_items;
DROP TABLE IF EXISTS expiry_policies;
-- +goose StatementEnd
