CREATE TYPE integration_type AS ENUM ('MQTT', 'POLLING', 'WEBHOOK');

CREATE TABLE gates (
    id                 UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID             NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name               TEXT             NOT NULL,
    integration_type   integration_type NOT NULL DEFAULT 'MQTT',
    integration_config JSONB            NOT NULL DEFAULT '{}',
    status             TEXT             NOT NULL DEFAULT 'unknown',
    last_seen_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gates_workspace ON gates(workspace_id);
