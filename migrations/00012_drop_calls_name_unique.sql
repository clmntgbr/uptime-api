-- +goose Up
-- Call names are labels only; identity is the call id. Duplicate names are allowed.
ALTER TABLE calls DROP CONSTRAINT IF EXISTS calls_workflow_name_key;

-- +goose Down
ALTER TABLE calls
    ADD CONSTRAINT calls_workflow_name_key UNIQUE (workflow_id, name);
