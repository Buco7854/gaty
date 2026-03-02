CREATE TABLE users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TYPE workspace_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER');

CREATE TABLE workspaces (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    owner_id      UUID        NOT NULL REFERENCES users(id),
    oidc_settings JSONB       NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX idx_workspaces_owner ON workspaces(owner_id);

CREATE TABLE workspace_members (
    workspace_id   UUID           NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id        UUID           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_role workspace_role NOT NULL DEFAULT 'MEMBER',
    PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX idx_workspace_members_user ON workspace_members(user_id);
