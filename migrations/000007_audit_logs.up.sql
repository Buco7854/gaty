CREATE TABLE audit_logs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    gate_id      UUID        REFERENCES gates(id) ON DELETE SET NULL,
    user_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    action       TEXT        NOT NULL,
    ip_address   TEXT,
    metadata     JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary query: "give me the last N events for this workspace"
CREATE INDEX idx_audit_logs_workspace_time ON audit_logs(workspace_id, created_at DESC);

-- Secondary query: "give me the last N events for this gate"
CREATE INDEX idx_audit_logs_gate_time ON audit_logs(gate_id, created_at DESC) WHERE gate_id IS NOT NULL;
