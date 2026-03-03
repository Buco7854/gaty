CREATE TABLE custom_domains (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    gate_id             UUID        NOT NULL REFERENCES gates(id)      ON DELETE CASCADE,
    workspace_id        UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain              TEXT        NOT NULL UNIQUE,
    -- Token to be placed as TXT record at _gaty.<domain>
    dns_challenge_token TEXT        NOT NULL DEFAULT encode(gen_random_bytes(24), 'hex'),
    verified_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_custom_domains_gate   ON custom_domains(gate_id);
CREATE INDEX idx_custom_domains_domain ON custom_domains(domain);
