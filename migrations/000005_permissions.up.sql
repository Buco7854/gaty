CREATE TABLE permissions (
    code        TEXT PRIMARY KEY,
    description TEXT NOT NULL
);

INSERT INTO permissions (code, description) VALUES
    ('gate:read_status',  'View gate status and details'),
    ('gate:trigger_open', 'Send open command to a gate'),
    ('gate:manage',       'Manage gate configuration and credentials'),
    ('workspace:manage',  'Manage workspace settings and members');

CREATE TABLE gate_user_policies (
    gate_id         UUID NOT NULL REFERENCES gates(id)        ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id)        ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (gate_id, user_id, permission_code)
);

-- Fast lookup for "which gates can this user see?"
CREATE INDEX idx_gate_user_policies_user ON gate_user_policies(user_id);
