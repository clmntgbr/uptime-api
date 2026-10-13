-- +goose Up
-- +goose StatementBegin
-- One NOLOGIN group role per service, with the least privileges it needs.
-- Login users are created by operations and granted the matching role
-- (example: CREATE USER scheduler_app LOGIN PASSWORD '...' IN ROLE uptime_scheduler).
-- Skipped when the migration user cannot manage roles (managed databases).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles WHERE rolname = current_user AND (rolsuper OR rolcreaterole)
    ) THEN
        RAISE NOTICE 'current user cannot create roles: skipping service roles';
        RETURN;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        CREATE ROLE uptime_api NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_scheduler') THEN
        CREATE ROLE uptime_scheduler NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_worker') THEN
        CREATE ROLE uptime_worker NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_cli') THEN
        CREATE ROLE uptime_cli NOLOGIN;
    END IF;

    GRANT USAGE ON SCHEMA public TO uptime_api, uptime_scheduler, uptime_worker, uptime_cli;
    GRANT SELECT ON goose_db_version TO uptime_api, uptime_scheduler, uptime_worker, uptime_cli;

    -- api: configuration read/write, results read-only.
    GRANT SELECT, INSERT, UPDATE, DELETE ON users, outbox_events, workflows, calls, rules, variables, workflow_state TO uptime_api;
    GRANT SELECT ON runs, call_runs, rule_runs, certificates TO uptime_api;

    -- cli: same as api (imports workflows and global variables).
    GRANT SELECT, INSERT, UPDATE, DELETE ON users, outbox_events, workflows, calls, rules, variables, workflow_state TO uptime_cli;
    GRANT SELECT ON runs, call_runs, rule_runs, certificates TO uptime_cli;

    -- scheduler: reads configuration, leases workflow_state, records skipped/timeout runs, deletes expired runs.
    GRANT SELECT ON users, workflows, calls, rules, variables TO uptime_scheduler;
    GRANT SELECT, UPDATE ON workflow_state TO uptime_scheduler;
    GRANT SELECT, INSERT, DELETE ON runs, call_runs, rule_runs TO uptime_scheduler;
    GRANT SELECT, INSERT ON processed_events, outbox_events TO uptime_scheduler;

    -- worker: records results, updates state, relays the outbox, dedups handlers.
    GRANT SELECT ON users, workflows TO uptime_worker;
    GRANT SELECT, UPDATE ON workflow_state TO uptime_worker;
    GRANT SELECT, INSERT ON runs, call_runs, rule_runs TO uptime_worker;
    GRANT SELECT, INSERT, UPDATE ON certificates TO uptime_worker;
    GRANT SELECT, INSERT, UPDATE, DELETE ON processed_events, outbox_events TO uptime_worker;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    role_name TEXT;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['uptime_api', 'uptime_scheduler', 'uptime_worker', 'uptime_cli'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('REASSIGN OWNED BY %I TO current_user', role_name);
            EXECUTE format('DROP OWNED BY %I', role_name);
            EXECUTE format('DROP ROLE %I', role_name);
        END IF;
    END LOOP;
END;
$$;
-- +goose StatementEnd
