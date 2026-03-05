ALTER TABLE gates DROP CONSTRAINT IF EXISTS gates_gate_token_unique;
ALTER TABLE gates
    DROP COLUMN IF EXISTS gate_token,
    DROP COLUMN IF EXISTS status_metadata,
    DROP COLUMN IF EXISTS meta_config;
