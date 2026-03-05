CREATE TABLE permissions (
    code        TEXT PRIMARY KEY,
    description TEXT NOT NULL
);

INSERT INTO permissions (code, description) VALUES
    ('gate:read_status',   'View gate status and details'),
    ('gate:trigger_open',  'Send open command to a gate'),
    ('gate:trigger_close', 'Send close command to a gate'),
    ('gate:manage',        'Manage gate configuration and access codes');

-- Gate access policies tied to membership_id (not user identity directly).
-- This ensures permissions survive a local→global merge without any data migration.
CREATE TABLE membership_policies (
    membership_id   UUID NOT NULL REFERENCES workspace_memberships(id) ON DELETE CASCADE,
    gate_id         UUID NOT NULL REFERENCES gates(id)                 ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES permissions(code)         ON DELETE CASCADE,
    PRIMARY KEY (membership_id, gate_id, permission_code)
);

CREATE INDEX idx_membership_policies_membership ON membership_policies(membership_id);
CREATE INDEX idx_membership_policies_gate ON membership_policies(gate_id);

-- Optional time-restriction schedule for a member's access to a specific gate.
-- A single schedule per (membership, gate) pair. If absent, access is unrestricted (time-wise).
CREATE TABLE membership_gate_schedules (
    membership_id UUID NOT NULL REFERENCES workspace_memberships(id) ON DELETE CASCADE,
    gate_id       UUID NOT NULL REFERENCES gates(id)                 ON DELETE CASCADE,
    schedule_id   UUID NOT NULL REFERENCES access_schedules(id)      ON DELETE CASCADE,
    PRIMARY KEY (membership_id, gate_id)
);
