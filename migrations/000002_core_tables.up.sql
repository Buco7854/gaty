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

CREATE TYPE integration_type AS ENUM ('MQTT', 'POLLING', 'WEBHOOK');

CREATE TABLE gates (
    id                 UUID             PRIMARY KEY DEFAULT uuidv7(),
    workspace_id       UUID             NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name               TEXT             NOT NULL,
    integration_type   integration_type NOT NULL DEFAULT 'MQTT',
    integration_config JSONB            NOT NULL DEFAULT '{}',
    status             TEXT             NOT NULL DEFAULT 'unknown',
    last_seen_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    status_rules JSONB NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_gates_workspace ON gates(workspace_id);

-- Partial index for efficient TTL scans: only indexes gates that can still
-- become unresponsive (excludes already-unresponsive and never-seen gates).
-- The TTL worker queries: last_seen_at < NOW()-ttl AND status NOT IN ('unresponsive','unknown')
-- This index makes that scan O(k) where k = number of active gates with a timestamp.
CREATE INDEX IF NOT EXISTS idx_gates_ttl_candidates
    ON gates (last_seen_at)
    WHERE last_seen_at IS NOT NULL
      AND status NOT IN ('unresponsive', 'unknown');

-- Credentials for platform users (PASSWORD, SSO_IDENTITY, API_TOKEN)
CREATE TABLE credentials (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type         TEXT        NOT NULL CHECK (type IN ('PASSWORD', 'SSO_IDENTITY', 'API_TOKEN')),
    hashed_value TEXT        NOT NULL,
    label        TEXT,
    expires_at   TIMESTAMPTZ,
    metadata     JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One password per user
CREATE UNIQUE INDEX idx_credentials_one_password ON credentials(user_id) WHERE type = 'PASSWORD';
-- Fast lookup by user + type
CREATE INDEX idx_credentials_user_type ON credentials(user_id, type);

-- Credentials for managed members (same structure, tied to membership)
CREATE TABLE membership_credentials (
    id             UUID        PRIMARY KEY DEFAULT uuidv7(),
    membership_id  UUID        NOT NULL REFERENCES workspace_memberships(id) ON DELETE CASCADE,
    type           TEXT        NOT NULL CHECK (type IN ('PASSWORD', 'SSO_IDENTITY', 'API_TOKEN')),
    hashed_value   TEXT        NOT NULL,
    label          TEXT,
    expires_at     TIMESTAMPTZ,
    metadata       JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One password per membership
CREATE UNIQUE INDEX idx_membership_creds_one_password ON membership_credentials(membership_id) WHERE type = 'PASSWORD';
-- Fast lookup by membership + type
CREATE INDEX idx_membership_creds_type ON membership_credentials(membership_id, type);

-- Reusable time-restriction schedules. Can be attached to PINs or member-gate access.
-- expr is a JSONB boolean expression tree (ExprNode). NULL means always allowed.
-- Nodes: { "op":"and"|"or"|"not", "children":[...] } or { "op":"rule", "rule":{...} }
-- Access is granted when expr evaluates to true. Use NOT node to invert.
-- membership_id IS NULL  → workspace schedule (admin-managed, shared, assignable to pins/members)
-- membership_id IS NOT NULL → member schedule (personal, only usable by that member on their own tokens)
CREATE TABLE access_schedules (
    id            UUID        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id  UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    membership_id UUID        REFERENCES workspace_memberships(id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    description   TEXT,
    expr          JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_access_schedules_workspace   ON access_schedules(workspace_id);
CREATE INDEX idx_access_schedules_membership  ON access_schedules(membership_id) WHERE membership_id IS NOT NULL;

-- Access codes for gates (PINs and passwords, separate concept from user/member credentials)
CREATE TABLE gate_access_codes (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    gate_id     UUID        NOT NULL REFERENCES gates(id) ON DELETE CASCADE,
    hashed_pin  TEXT        NOT NULL,
    label       TEXT        NOT NULL,
    metadata    JSONB       NOT NULL DEFAULT '{}',
    schedule_id UUID        REFERENCES access_schedules(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gate_access_codes_gate ON gate_access_codes(gate_id);

CREATE TABLE permissions (
    code        TEXT PRIMARY KEY,
    description TEXT NOT NULL
);

INSERT INTO permissions (code, description) VALUES
    ('gate:read_status',   'View gate status and details'),
    ('gate:trigger_open',  'Send open command to a gate'),
    ('gate:trigger_close', 'Send close command to a gate'),
    ('gate:manage',        'Manage gate configuration and access codes');

-- Unified access policy table — covers memberships AND API credentials.
-- subject_type = 'membership' → subject_id = workspace_memberships.id
-- subject_type = 'credential' → subject_id = membership_credentials.id
CREATE TABLE access_policies (
    subject_type    TEXT NOT NULL CHECK (subject_type IN ('membership', 'credential')),
    subject_id      UUID NOT NULL,
    gate_id         UUID NOT NULL REFERENCES gates(id)         ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (subject_type, subject_id, gate_id, permission_code)
);

CREATE INDEX idx_access_policies_subject ON access_policies(subject_type, subject_id);
CREATE INDEX idx_access_policies_gate    ON access_policies(gate_id);

-- Unified schedule link table — covers memberships AND API credentials.
-- gate_id NULL  = global restriction on a credential (applies to all gates)
-- gate_id SET   = gate-specific restriction (used for membership-gate schedules)
CREATE TABLE schedule_links (
    subject_type TEXT NOT NULL CHECK (subject_type IN ('membership', 'credential')),
    subject_id   UUID NOT NULL,
    gate_id      UUID REFERENCES gates(id) ON DELETE CASCADE,
    schedule_id  UUID NOT NULL REFERENCES access_schedules(id) ON DELETE CASCADE
);

-- One schedule per (subject, gate) when gate is specified (membership-gate schedules)
CREATE UNIQUE INDEX uq_schedule_links_gate ON schedule_links(subject_type, subject_id, gate_id)
    WHERE gate_id IS NOT NULL;
-- One global schedule per subject (for credential-level restrictions)
CREATE UNIQUE INDEX uq_schedule_links_global ON schedule_links(subject_type, subject_id)
    WHERE gate_id IS NULL;

CREATE TABLE custom_domains (
    id                  UUID        PRIMARY KEY DEFAULT uuidv7(),
    gate_id             UUID        NOT NULL REFERENCES gates(id)      ON DELETE CASCADE,
    workspace_id        UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain              TEXT        NOT NULL UNIQUE,
    -- Token to be placed as TXT record at _gatie.<domain>
    dns_challenge_token TEXT        NOT NULL DEFAULT encode(gen_random_bytes(24), 'hex'),
    verified_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_custom_domains_gate   ON custom_domains(gate_id);
CREATE INDEX idx_custom_domains_domain ON custom_domains(domain);

-- Per-action integration configs for gates
-- Replaces the single integration_type/integration_config with three independent configs.

ALTER TABLE gates
    ADD COLUMN open_config   JSONB,
    ADD COLUMN close_config  JSONB,
    ADD COLUMN status_config JSONB;

-- Migrate existing MQTT gates: open and status use MQTT_GATIE by default
UPDATE gates
SET open_config   = '{"type": "MQTT_GATIE"}'::jsonb,
    status_config = '{"type": "MQTT_GATIE"}'::jsonb
WHERE integration_type = 'MQTT';

-- Gate token: unique authentication token per gate.
-- Gates include this token in every status payload so the server can validate their identity.
ALTER TABLE gates
    ADD COLUMN gate_token        TEXT NOT NULL DEFAULT encode(gen_random_bytes(32), 'hex'),
    ADD COLUMN status_metadata   JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN meta_config       JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN custom_statuses   JSONB NOT NULL DEFAULT '[]';

-- Ensure token uniqueness (DEFAULT already generates unique values per row).
ALTER TABLE gates ADD CONSTRAINT gates_gate_token_unique UNIQUE (gate_token);
