-- +goose Up
-- +goose StatementBegin
-- Virtual / stored defaults: disabled with empty thresholds (configure at create).
ALTER TABLE expiry_policies
    ALTER COLUMN enabled SET DEFAULT FALSE,
    ALTER COLUMN thresholds_days SET DEFAULT '[]'::jsonb;

ALTER TABLE expiry_policies
    DROP CONSTRAINT expiry_policies_thresholds_len;

ALTER TABLE expiry_policies
    ADD CONSTRAINT expiry_policies_thresholds_len CHECK (
        jsonb_typeof(thresholds_days) = 'array'
        AND jsonb_array_length(thresholds_days) BETWEEN 0 AND 20
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Empty arrays are valid after 00014; fill them before restoring the old CHECK.
UPDATE expiry_policies
SET thresholds_days = '[30,14,7,1]'::jsonb
WHERE jsonb_array_length(thresholds_days) = 0;

ALTER TABLE expiry_policies
    DROP CONSTRAINT expiry_policies_thresholds_len;

ALTER TABLE expiry_policies
    ADD CONSTRAINT expiry_policies_thresholds_len CHECK (
        jsonb_typeof(thresholds_days) = 'array'
        AND jsonb_array_length(thresholds_days) BETWEEN 1 AND 20
    );

ALTER TABLE expiry_policies
    ALTER COLUMN enabled SET DEFAULT TRUE,
    ALTER COLUMN thresholds_days SET DEFAULT '[30,14,7,1]'::jsonb;
-- +goose StatementEnd
