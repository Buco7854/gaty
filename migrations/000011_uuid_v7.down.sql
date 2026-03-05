ALTER TABLE users                  ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE workspaces             ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE workspace_memberships  ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE credentials            ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE membership_credentials ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE access_schedules       ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE gate_access_codes      ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE gates                  ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE custom_domains         ALTER COLUMN id SET DEFAULT gen_random_uuid();

DROP FUNCTION IF EXISTS uuid_generate_v7();
