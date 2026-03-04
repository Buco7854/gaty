-- Credentials for platform users (PASSWORD, SSO_IDENTITY, API_TOKEN)
CREATE TABLE credentials (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
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
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
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

-- Access codes for gates (PINs and passwords, separate concept from user/member credentials)
CREATE TABLE gate_access_codes (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    gate_id    UUID        NOT NULL REFERENCES gates(id) ON DELETE CASCADE,
    hashed_pin TEXT        NOT NULL,
    label      TEXT        NOT NULL,
    metadata   JSONB       NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gate_access_codes_gate ON gate_access_codes(gate_id);
