CREATE TABLE users (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    email      TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE workspaces (
    id                 UUID        PRIMARY KEY DEFAULT uuidv7(),
    name               TEXT        NOT NULL,
    owner_id           UUID        NOT NULL REFERENCES users(id),
    sso_settings       JSONB       NOT NULL DEFAULT '{}',
    member_auth_config JSONB       NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workspaces_owner ON workspaces(owner_id);

-- Every person with workspace access: platform users (user_id set) and managed members (user_id null).
CREATE TABLE workspace_memberships (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    local_username TEXT,
    display_name TEXT,
    role         TEXT        NOT NULL DEFAULT 'MEMBER' CHECK (role IN ('OWNER', 'ADMIN', 'MEMBER')),
    auth_config  JSONB       NOT NULL DEFAULT '{}',
    invited_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- must have a platform account OR a local username to identify them
    CONSTRAINT chk_has_identity CHECK (user_id IS NOT NULL OR local_username IS NOT NULL),
    -- one membership per user per workspace
    CONSTRAINT uq_workspace_user UNIQUE (workspace_id, user_id),
    -- unique local username per workspace (partial: only where set)
    CONSTRAINT uq_workspace_local_username UNIQUE (workspace_id, local_username)
);

CREATE INDEX idx_memberships_workspace ON workspace_memberships(workspace_id);
CREATE INDEX idx_memberships_user ON workspace_memberships(user_id) WHERE user_id IS NOT NULL;
