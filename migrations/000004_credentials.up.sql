CREATE TYPE credential_target_type AS ENUM ('USER', 'MEMBER', 'GATE');
CREATE TYPE credential_type AS ENUM ('PASSWORD', 'PIN_CODE', 'API_TOKEN', 'OIDC_IDENTITY');

CREATE TABLE credentials (
    id              UUID                   PRIMARY KEY DEFAULT gen_random_uuid(),
    target_type     credential_target_type NOT NULL,
    target_id       UUID                   NOT NULL,
    credential_type credential_type        NOT NULL,
    hashed_value    TEXT                   NOT NULL,
    metadata        JSONB                  NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ            NOT NULL DEFAULT NOW()
);

-- One PASSWORD per user/gate target
CREATE UNIQUE INDEX idx_credentials_unique_password
    ON credentials(target_type, target_id)
    WHERE credential_type = 'PASSWORD';

-- Fast lookup by target (used on every auth check)
CREATE INDEX idx_credentials_target
    ON credentials(target_type, target_id, credential_type);
