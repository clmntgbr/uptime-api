-- +goose Up
-- +goose StatementBegin
-- Result tables grow fast: they are range-partitioned by month on the run start
-- time, written only by the worker (and the scheduler for skipped/timeout runs),
-- and expired by dropping whole partitions. Run idempotence is enforced through
-- processed_events (event_id = run id), because a unique key on a partitioned
-- table must include the partition key.
CREATE TABLE IF NOT EXISTS runs (
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

CREATE INDEX IF NOT EXISTS idx_runs_workflow_started ON runs (workflow_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_user_started ON runs (user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_id ON runs (id);

CREATE TABLE IF NOT EXISTS call_results (
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

CREATE INDEX IF NOT EXISTS idx_call_results_run ON call_results (run_id, position);

CREATE TABLE IF NOT EXISTS rule_results (
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

CREATE INDEX IF NOT EXISTS idx_rule_results_call_result ON rule_results (call_result_id, position);

CREATE TABLE IF NOT EXISTS hourly_aggregates (
    workflow_id UUID NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    runs_total INTEGER NOT NULL,
    runs_success INTEGER NOT NULL,
    availability NUMERIC(6, 3) NOT NULL,
    latency_p50_ms INTEGER NULL,
    latency_p95_ms INTEGER NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workflow_id, bucket_start)
);

CREATE TABLE IF NOT EXISTS daily_aggregates (
    workflow_id UUID NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    runs_total INTEGER NOT NULL,
    runs_success INTEGER NOT NULL,
    availability NUMERIC(6, 3) NOT NULL,
    latency_p50_ms INTEGER NULL,
    latency_p95_ms INTEGER NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workflow_id, bucket_start)
);

-- Creating or dropping a partition requires owning the parent table. These two
-- functions run as their owner so the scheduler role only needs EXECUTE.
CREATE OR REPLACE FUNCTION ensure_result_partitions(months_ahead INTEGER)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
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
$$;

CREATE OR REPLACE FUNCTION drop_result_partitions_before(cutoff TIMESTAMPTZ)
RETURNS SETOF TEXT
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
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
$$;

SELECT ensure_result_partitions(2);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS drop_result_partitions_before(TIMESTAMPTZ);
DROP FUNCTION IF EXISTS ensure_result_partitions(INTEGER);
DROP TABLE IF EXISTS daily_aggregates;
DROP TABLE IF EXISTS hourly_aggregates;
DROP TABLE IF EXISTS rule_results;
DROP TABLE IF EXISTS call_results;
DROP TABLE IF EXISTS runs;
-- +goose StatementEnd
