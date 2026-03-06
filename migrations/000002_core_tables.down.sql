ALTER TABLE gates DROP CONSTRAINT IF EXISTS gates_gate_token_unique;
ALTER TABLE gates
    DROP COLUMN IF EXISTS gate_token,
    DROP COLUMN IF EXISTS status_metadata,
    DROP COLUMN IF EXISTS meta_config;

ALTER TABLE gates
    DROP COLUMN IF EXISTS open_config,
    DROP COLUMN IF EXISTS close_config,
    DROP COLUMN IF EXISTS status_config;

DROP TABLE IF EXISTS custom_domains;

DROP TABLE IF EXISTS schedule_links;
DROP TABLE IF EXISTS access_policies;
DROP TABLE IF EXISTS permissions;

DROP TABLE IF EXISTS gate_access_codes;
DROP TABLE IF EXISTS membership_credentials;
DROP TABLE IF EXISTS credentials;

ALTER TABLE gates DROP COLUMN IF EXISTS status_rules;
DROP INDEX IF EXISTS idx_gates_ttl_candidates;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS gates;
DROP TYPE IF EXISTS integration_type;

DROP TABLE IF EXISTS workspace_memberships CASCADE;
DROP TABLE IF EXISTS workspace_members CASCADE;
DROP TABLE IF EXISTS members CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;
DROP TABLE IF EXISTS users CASCADE;
