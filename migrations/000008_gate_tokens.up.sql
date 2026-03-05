-- Gate token: unique authentication token per gate.
-- Gates include this token in every status payload so the server can validate their identity.
ALTER TABLE gates
    ADD COLUMN gate_token      TEXT NOT NULL DEFAULT encode(gen_random_bytes(32), 'hex'),
    ADD COLUMN status_metadata JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN meta_config     JSONB NOT NULL DEFAULT '[]';

-- Ensure token uniqueness (DEFAULT already generates unique values per row).
ALTER TABLE gates ADD CONSTRAINT gates_gate_token_unique UNIQUE (gate_token);
