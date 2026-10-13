-- +goose Up
-- +goose StatementBegin
-- Result tables become plain tables: call_results is renamed call_runs, the
-- run_started_at partition key disappears, and children are deleted with their
-- run through foreign keys, so the retention only deletes old runs.
ALTER TABLE runs RENAME TO runs_partitioned;
ALTER TABLE call_results RENAME TO call_results_partitioned;
ALTER TABLE rule_results RENAME TO rule_results_partitioned;
ALTER INDEX runs_pkey RENAME TO runs_partitioned_pkey;
ALTER INDEX rule_results_pkey RENAME TO rule_results_partitioned_pkey;
DROP INDEX IF EXISTS idx_runs_workflow_started;
DROP INDEX IF EXISTS idx_runs_user_started;
DROP INDEX IF EXISTS idx_runs_id;

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

CREATE TABLE rule_results (
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

CREATE INDEX idx_rule_results_call_run ON rule_results (call_run_id, position);
CREATE INDEX idx_rule_results_run ON rule_results (run_id);

INSERT INTO runs
SELECT DISTINCT ON (id)
    id, workflow_id, user_id, trigger, status, reason, started_at, finished_at, duration_ms,
    calls_total, calls_success, calls_failed, calls_error, calls_skipped, variables, recorded_at
FROM runs_partitioned
ORDER BY id, recorded_at DESC;

INSERT INTO call_runs
SELECT DISTINCT ON (c.id)
    c.id, c.run_id, c.workflow_id, c.call_id, c.name, c.position, c.status, c.reason, c.http_status,
    c.request, c.response, c.timings, c.network, c.tls, c.extracted_variables, c.used_variables,
    c.error_type, c.error_message, c.body_ref, c.truncated, c.started_at, c.finished_at
FROM call_results_partitioned c
JOIN runs r ON r.id = c.run_id
ORDER BY c.id;

INSERT INTO rule_results
SELECT DISTINCT ON (rr.id)
    rr.id, rr.call_result_id, rr.run_id, rr.position, rr.subject, rr.operator,
    rr.text, rr.observed, rr.expected, rr.passed
FROM rule_results_partitioned rr
JOIN call_runs c ON c.id = rr.call_result_id
ORDER BY rr.id;

DROP TABLE rule_results_partitioned;
DROP TABLE call_results_partitioned;
DROP TABLE runs_partitioned;
DROP FUNCTION IF EXISTS drop_result_partitions_before(TIMESTAMPTZ);
DROP FUNCTION IF EXISTS ensure_result_partitions(INTEGER);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        RETURN;
    END IF;
    GRANT SELECT ON runs, call_runs, rule_results TO uptime_api, uptime_cli;
    -- scheduler: records skipped/timeout runs and deletes the expired ones.
    GRANT SELECT, INSERT, DELETE ON runs, call_runs, rule_results TO uptime_scheduler;
    GRANT SELECT, INSERT ON runs, call_runs, rule_results TO uptime_worker;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE rule_results RENAME TO rule_results_plain;
ALTER TABLE call_runs RENAME TO call_runs_plain;
ALTER TABLE runs RENAME TO runs_plain;
ALTER INDEX runs_pkey RENAME TO runs_plain_pkey;
ALTER INDEX rule_results_pkey RENAME TO rule_results_plain_pkey;
DROP INDEX IF EXISTS idx_runs_workflow_started;
DROP INDEX IF EXISTS idx_runs_user_started;
DROP INDEX IF EXISTS idx_runs_started;

CREATE TABLE runs (
    id UUID NOT NULL,
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
    PRIMARY KEY (id, started_at),
    CONSTRAINT runs_status CHECK (status IN ('success', 'failed', 'skipped', 'error')),
    CONSTRAINT runs_trigger CHECK (trigger IN ('schedule', 'manual', 'test'))
) PARTITION BY RANGE (started_at);

CREATE INDEX idx_runs_workflow_started ON runs (workflow_id, started_at DESC);
CREATE INDEX idx_runs_user_started ON runs (user_id, started_at DESC);
CREATE INDEX idx_runs_id ON runs (id);

CREATE TABLE call_results (
    id UUID NOT NULL,
    run_id UUID NOT NULL,
    run_started_at TIMESTAMPTZ NOT NULL,
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
    PRIMARY KEY (id, run_started_at),
    CONSTRAINT call_results_status CHECK (status IN ('success', 'failed', 'error', 'skipped'))
) PARTITION BY RANGE (run_started_at);

CREATE INDEX idx_call_results_run ON call_results (run_id, position);

CREATE TABLE rule_results (
    id UUID NOT NULL,
    call_result_id UUID NOT NULL,
    run_id UUID NOT NULL,
    run_started_at TIMESTAMPTZ NOT NULL,
    position INTEGER NOT NULL,
    subject TEXT NOT NULL,
    operator TEXT NOT NULL,
    text TEXT NOT NULL,
    observed TEXT NOT NULL,
    expected TEXT NOT NULL,
    passed BOOLEAN NOT NULL,
    PRIMARY KEY (id, run_started_at)
) PARTITION BY RANGE (run_started_at);

CREATE INDEX idx_rule_results_call_result ON rule_results (call_result_id, position);

CREATE OR REPLACE FUNCTION ensure_result_partitions(months_ahead INTEGER)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $fn$
DECLARE
    parent TEXT;
    i INTEGER;
    month_start DATE;
BEGIN
    FOREACH parent IN ARRAY ARRAY['runs', 'call_results', 'rule_results'] LOOP
        FOR i IN 0..months_ahead LOOP
            month_start := (date_trunc('month', NOW() AT TIME ZONE 'UTC') + make_interval(months => i))::DATE;
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
                parent || '_p' || to_char(month_start, 'YYYY_MM'),
                parent,
                to_char(month_start, 'YYYY-MM-DD') || ' 00:00:00+00',
                to_char((month_start + INTERVAL '1 month')::DATE, 'YYYY-MM-DD') || ' 00:00:00+00'
            );
        END LOOP;
    END LOOP;
END;
$fn$;

CREATE OR REPLACE FUNCTION drop_result_partitions_before(cutoff TIMESTAMPTZ)
RETURNS SETOF TEXT
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $fn$
DECLARE
    part RECORD;
    upper_bound TIMESTAMPTZ;
BEGIN
    FOR part IN
        SELECT child.relname AS name
        FROM pg_inherits inh
        JOIN pg_class child ON child.oid = inh.inhrelid
        JOIN pg_class parent ON parent.oid = inh.inhparent
        WHERE parent.relname IN ('runs', 'call_results', 'rule_results')
          AND child.relname ~ '_p[0-9]{4}_[0-9]{2}$'
    LOOP
        upper_bound := (to_date(right(part.name, 7), 'YYYY_MM') + INTERVAL '1 month') AT TIME ZONE 'UTC';
        IF upper_bound <= cutoff THEN
            EXECUTE format('DROP TABLE IF EXISTS %I', part.name);
            RETURN NEXT part.name;
        END IF;
    END LOOP;
END;
$fn$;

-- Partitions must cover every copied row, not only the coming months.
DO $$
DECLARE
    month_start DATE;
    parent TEXT;
BEGIN
    PERFORM ensure_result_partitions(2);
    FOR month_start IN
        SELECT DISTINCT date_trunc('month', started_at AT TIME ZONE 'UTC')::DATE FROM runs_plain
    LOOP
        FOREACH parent IN ARRAY ARRAY['runs', 'call_results', 'rule_results'] LOOP
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
                parent || '_p' || to_char(month_start, 'YYYY_MM'),
                parent,
                to_char(month_start, 'YYYY-MM-DD') || ' 00:00:00+00',
                to_char((month_start + INTERVAL '1 month')::DATE, 'YYYY-MM-DD') || ' 00:00:00+00'
            );
        END LOOP;
    END LOOP;
END;
$$;

INSERT INTO runs SELECT * FROM runs_plain;
INSERT INTO call_results
SELECT c.id, c.run_id, r.started_at, c.workflow_id, c.call_id, c.name, c.position, c.status, c.reason,
    c.http_status, c.request, c.response, c.timings, c.network, c.tls, c.extracted_variables,
    c.used_variables, c.error_type, c.error_message, c.body_ref, c.truncated, c.started_at, c.finished_at
FROM call_runs_plain c
JOIN runs_plain r ON r.id = c.run_id;
INSERT INTO rule_results
SELECT rr.id, rr.call_run_id, rr.run_id, r.started_at, rr.position, rr.subject, rr.operator,
    rr.text, rr.observed, rr.expected, rr.passed
FROM rule_results_plain rr
JOIN runs_plain r ON r.id = rr.run_id;

DROP TABLE rule_results_plain;
DROP TABLE call_runs_plain;
DROP TABLE runs_plain;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'uptime_api') THEN
        RETURN;
    END IF;
    GRANT SELECT ON runs, call_results, rule_results TO uptime_api, uptime_cli;
    GRANT SELECT, INSERT ON runs TO uptime_scheduler;
    GRANT SELECT, INSERT ON runs, call_results, rule_results TO uptime_worker;
    GRANT EXECUTE ON FUNCTION ensure_result_partitions(INTEGER) TO uptime_scheduler;
    GRANT EXECUTE ON FUNCTION drop_result_partitions_before(TIMESTAMPTZ) TO uptime_scheduler;
    REVOKE EXECUTE ON FUNCTION ensure_result_partitions(INTEGER) FROM PUBLIC;
    REVOKE EXECUTE ON FUNCTION drop_result_partitions_before(TIMESTAMPTZ) FROM PUBLIC;
END;
$$;
-- +goose StatementEnd
