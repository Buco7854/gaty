-- Members: the only identity table. Each person is a member with a username.
CREATE TABLE members (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    username     TEXT        NOT NULL UNIQUE,
    display_name TEXT,
    role         TEXT        NOT NULL DEFAULT 'MEMBER' CHECK (role IN ('ADMIN', 'MEMBER')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Credentials for members (PASSWORD, SSO_IDENTITY, API_TOKEN)
CREATE TABLE credentials (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    member_id    UUID        NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    type         TEXT        NOT NULL CHECK (type IN ('PASSWORD', 'SSO_IDENTITY', 'API_TOKEN')),
    hashed_value TEXT        NOT NULL,
    label        TEXT,
    expires_at   TIMESTAMPTZ,
    metadata     JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One password per member
CREATE UNIQUE INDEX idx_credentials_one_password ON credentials(member_id) WHERE type = 'PASSWORD';
-- Fast lookup by member + type
CREATE INDEX idx_credentials_member_type ON credentials(member_id, type);

CREATE TYPE integration_type AS ENUM ('MQTT', 'POLLING', 'WEBHOOK');

CREATE TABLE gates (
    id                 UUID             PRIMARY KEY DEFAULT uuidv7(),
    name               TEXT             NOT NULL,
    integration_type   integration_type NOT NULL DEFAULT 'MQTT',
    integration_config JSONB            NOT NULL DEFAULT '{}',
    status             TEXT             NOT NULL DEFAULT 'unknown',
    last_seen_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    status_rules       JSONB            NOT NULL DEFAULT '[]',
    open_config        JSONB,
    close_config       JSONB,
    status_config      JSONB,
    gate_token         TEXT             NOT NULL DEFAULT encode(gen_random_bytes(32), 'hex'),
    status_metadata    JSONB            NOT NULL DEFAULT '{}',
    meta_config        JSONB            NOT NULL DEFAULT '[]',
    custom_statuses    JSONB            NOT NULL DEFAULT '[]',
    ttl_seconds        INTEGER,
    status_transitions JSONB            NOT NULL DEFAULT '[]',
    CONSTRAINT gates_gate_token_unique UNIQUE (gate_token)
);

-- Partial index for efficient TTL scans.
CREATE INDEX IF NOT EXISTS idx_gates_ttl_candidates
    ON gates (last_seen_at)
    WHERE last_seen_at IS NOT NULL
      AND status NOT IN ('unresponsive', 'unknown');

-- Reusable time-restriction schedules.
-- member_id IS NULL  = shared schedule (admin-managed, assignable to pins/members)
-- member_id IS NOT NULL = personal schedule (only usable by that member on their own tokens)
CREATE TABLE access_schedules (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    member_id   UUID        REFERENCES members(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT,
    expr        JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_access_schedules_member ON access_schedules(member_id) WHERE member_id IS NOT NULL;

-- Access codes for gates (PINs and passwords)
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

-- Unified access policy table: membership and credential subjects.
-- subject_type = 'membership' -> subject_id = members.id
-- subject_type = 'credential' -> subject_id = credentials.id
CREATE TABLE access_policies (
    subject_type    TEXT NOT NULL CHECK (subject_type IN ('membership', 'credential')),
    subject_id      UUID NOT NULL,
    gate_id         UUID NOT NULL REFERENCES gates(id)         ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (subject_type, subject_id, gate_id, permission_code)
);

CREATE INDEX idx_access_policies_subject ON access_policies(subject_type, subject_id);
CREATE INDEX idx_access_policies_gate    ON access_policies(gate_id);

-- Unified schedule link table.
-- gate_id NULL  = global restriction on a credential (applies to all gates)
-- gate_id SET   = gate-specific restriction (used for member-gate schedules)
CREATE TABLE schedule_links (
    subject_type TEXT NOT NULL CHECK (subject_type IN ('membership', 'credential')),
    subject_id   UUID NOT NULL,
    gate_id      UUID REFERENCES gates(id) ON DELETE CASCADE,
    schedule_id  UUID NOT NULL REFERENCES access_schedules(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_schedule_links_gate ON schedule_links(subject_type, subject_id, gate_id)
    WHERE gate_id IS NOT NULL;
CREATE UNIQUE INDEX uq_schedule_links_global ON schedule_links(subject_type, subject_id)
    WHERE gate_id IS NULL;

CREATE TABLE custom_domains (
    id                  UUID        PRIMARY KEY DEFAULT uuidv7(),
    gate_id             UUID        NOT NULL REFERENCES gates(id) ON DELETE CASCADE,
    domain              TEXT        NOT NULL UNIQUE,
    dns_challenge_token TEXT        NOT NULL DEFAULT encode(gen_random_bytes(24), 'hex'),
    verified_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_custom_domains_gate   ON custom_domains(gate_id);
CREATE INDEX idx_custom_domains_domain ON custom_domains(domain);
