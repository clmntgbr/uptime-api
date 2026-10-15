-- +goose Up
-- +goose StatementBegin
-- Call variables (extractions) live in `variables` like globals and statics.
ALTER TABLE variables
    ADD COLUMN IF NOT EXISTS call_id UUID NULL REFERENCES calls (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS source TEXT NULL,
    ADD COLUMN IF NOT EXISTS path TEXT NULL;

ALTER TABLE variables DROP CONSTRAINT IF EXISTS variables_type;
ALTER TABLE variables
    ADD CONSTRAINT variables_type CHECK (type IN ('global', 'static', 'call'));

ALTER TABLE variables DROP CONSTRAINT IF EXISTS variables_scope;
ALTER TABLE variables
    ADD CONSTRAINT variables_scope CHECK (
        (type = 'global' AND workflow_id IS NULL AND call_id IS NULL AND source IS NULL AND path IS NULL)
        OR (type = 'static' AND workflow_id IS NOT NULL AND call_id IS NULL AND source IS NULL AND path IS NULL)
        OR (
            type = 'call'
            AND workflow_id IS NOT NULL
            AND call_id IS NOT NULL
            AND value = ''
            AND source IS NOT NULL
            AND source IN ('json', 'header', 'status', 'regex')
        )
    );

DROP INDEX IF EXISTS uq_variables_static_name;

-- Backfill from legacy calls.extractions JSONB (before unique indexes so
-- legacy name clashes become a clear INSERT failure, not an index build error).
INSERT INTO variables (id, user_id, workflow_id, call_id, type, name, value, secret, source, path, created_at, updated_at)
SELECT
    gen_random_uuid(),
    src.user_id,
    src.workflow_id,
    src.call_id,
    'call',
    src.name,
    '',
    src.secret,
    src.source,
    src.path,
    NOW(),
    NOW()
FROM (
    SELECT DISTINCT ON (c.workflow_id, e.elem->>'name')
        w.user_id,
        c.workflow_id,
        c.id AS call_id,
        e.elem->>'name' AS name,
        COALESCE((e.elem->>'secret')::boolean, FALSE) AS secret,
        COALESCE(
            NULLIF(e.elem->>'source', ''),
            CASE WHEN COALESCE(e.elem->>'expression', '') = '' THEN 'status' ELSE 'json' END
        ) AS source,
        NULLIF(COALESCE(e.elem->>'expression', ''), '') AS path
    FROM calls c
    JOIN workflows w ON w.id = c.workflow_id
    CROSS JOIN LATERAL jsonb_array_elements(c.extractions) AS e(elem)
    WHERE jsonb_typeof(c.extractions) = 'array'
      AND jsonb_array_length(c.extractions) > 0
      AND COALESCE(e.elem->>'name', '') <> ''
    ORDER BY c.workflow_id, e.elem->>'name', c.position
) src
WHERE src.source IN ('json', 'header', 'status', 'regex')
  AND NOT EXISTS (
      SELECT 1 FROM variables v
      WHERE v.workflow_id = src.workflow_id
        AND v.type IN ('static', 'call')
        AND v.name = src.name
  );

UPDATE calls SET extractions = '[]'::jsonb
WHERE jsonb_typeof(extractions) = 'array' AND jsonb_array_length(extractions) > 0;

CREATE UNIQUE INDEX uq_variables_workflow_name ON variables (workflow_id, name)
    WHERE type IN ('static', 'call');
CREATE UNIQUE INDEX uq_variables_call_name ON variables (call_id, name)
    WHERE type = 'call';
CREATE INDEX IF NOT EXISTS idx_variables_call_id ON variables (call_id) WHERE type = 'call';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Restore extractions JSON from variables rows (best-effort).
UPDATE calls c
SET extractions = COALESCE(sub.payload, '[]'::jsonb)
FROM (
    SELECT
        call_id,
        jsonb_agg(
            jsonb_strip_nulls(jsonb_build_object(
                'name', name,
                'source', source,
                'expression', NULLIF(path, ''),
                'secret', CASE WHEN secret THEN TRUE ELSE NULL END
            ))
            ORDER BY name
        ) AS payload
    FROM variables
    WHERE type = 'call' AND call_id IS NOT NULL
    GROUP BY call_id
) sub
WHERE c.id = sub.call_id;

DELETE FROM variables WHERE type = 'call';

DROP INDEX IF EXISTS idx_variables_call_id;
DROP INDEX IF EXISTS uq_variables_call_name;
DROP INDEX IF EXISTS uq_variables_workflow_name;
CREATE UNIQUE INDEX uq_variables_static_name ON variables (workflow_id, name) WHERE type = 'static';

ALTER TABLE variables DROP CONSTRAINT IF EXISTS variables_scope;
ALTER TABLE variables
    ADD CONSTRAINT variables_scope CHECK (
        (type = 'global' AND workflow_id IS NULL)
        OR (type = 'static' AND workflow_id IS NOT NULL)
    );

ALTER TABLE variables DROP CONSTRAINT IF EXISTS variables_type;
ALTER TABLE variables
    ADD CONSTRAINT variables_type CHECK (type IN ('global', 'static'));

ALTER TABLE variables
    DROP COLUMN IF EXISTS path,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS call_id;
-- +goose StatementEnd
