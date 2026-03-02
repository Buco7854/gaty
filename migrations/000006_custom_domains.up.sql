CREATE TYPE domain_target_type AS ENUM ('WORKSPACE', 'GATE');

CREATE TABLE custom_domains (
    id          UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_name TEXT               NOT NULL UNIQUE,
    target_type domain_target_type NOT NULL,
    target_id   UUID               NOT NULL,
    base_path   TEXT               NOT NULL DEFAULT '/',
    is_verified BOOLEAN            NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ        NOT NULL DEFAULT NOW()
);

-- Used by the Caddy verify-domain endpoint (hot path)
CREATE INDEX idx_custom_domains_name ON custom_domains(domain_name);

-- Used to list domains attached to a workspace or gate
CREATE INDEX idx_custom_domains_target ON custom_domains(target_type, target_id);
