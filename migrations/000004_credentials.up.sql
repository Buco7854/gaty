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

-- Reusable time-restriction schedules. Can be attached to PINs or member-gate access.
-- rules is a JSONB array of schedule rule objects. A schedule passes if ANY rule matches
-- (OR logic). An empty array means always allowed.
--
-- Rule types:
--   time_range:    { "type":"time_range", "days":[1,2,3,4,5], "start_time":"10:00", "end_time":"12:00" }
--                  days = [0..6] (0=Sun), omit for all days.
--   weekdays_range:{ "type":"weekdays_range", "start_day":6, "end_day":0 }
--                  Inclusive range. Wraps around (Sat=6 → Sun=0 is the week-end range).
--   date_range:    { "type":"date_range", "start_date":"2026-01-01", "end_date":"2026-12-31" }
CREATE TABLE access_schedules (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    description  TEXT,
    rules        JSONB       NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_access_schedules_workspace ON access_schedules(workspace_id);

-- Access codes for gates (PINs and passwords, separate concept from user/member credentials)
CREATE TABLE gate_access_codes (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    gate_id     UUID        NOT NULL REFERENCES gates(id) ON DELETE CASCADE,
    hashed_pin  TEXT        NOT NULL,
    label       TEXT        NOT NULL,
    metadata    JSONB       NOT NULL DEFAULT '{}',
    schedule_id UUID        REFERENCES access_schedules(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gate_access_codes_gate ON gate_access_codes(gate_id);
