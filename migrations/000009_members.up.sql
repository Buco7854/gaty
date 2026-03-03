CREATE TABLE members (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    display_name TEXT        NOT NULL,
    email        TEXT,
    username     TEXT        NOT NULL,
    user_id      UUID        REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    UNIQUE (workspace_id, username)
);

CREATE INDEX idx_members_workspace ON members(workspace_id);
CREATE INDEX idx_members_user ON members(user_id) WHERE user_id IS NOT NULL;

CREATE TABLE gate_member_policies (
    gate_id         UUID NOT NULL REFERENCES gates(id)        ON DELETE CASCADE,
    member_id       UUID NOT NULL REFERENCES members(id)      ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (gate_id, member_id, permission_code)
);

CREATE INDEX idx_gate_member_policies_member ON gate_member_policies(member_id);
