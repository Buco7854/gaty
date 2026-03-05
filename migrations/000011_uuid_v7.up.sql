-- UUID v7: time-ordered UUID with millisecond precision prefix + random suffix.
-- Better than v4 (gen_random_uuid) for primary keys: monotone insertion order
-- reduces B-tree index fragmentation and page splits.
-- Conflict-free: random bits (62 bits) guarantee global uniqueness.
CREATE OR REPLACE FUNCTION uuid_generate_v7() RETURNS uuid
LANGUAGE plpgsql PARALLEL SAFE
AS $$
DECLARE
  unix_ts_ms bytea;
  uuid_bytes bytea;
BEGIN
  -- 6-byte millisecond timestamp
  unix_ts_ms = substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3);
  -- Start from a random UUID v4 base (provides the random bits)
  uuid_bytes = uuid_send(gen_random_uuid());
  -- Overlay the timestamp on the first 6 bytes
  uuid_bytes = overlay(uuid_bytes PLACING unix_ts_ms FROM 1 FOR 6);
  -- Set version bits to 0111 (v7)
  uuid_bytes = set_byte(uuid_bytes, 6, (b'0111' || get_byte(uuid_bytes, 6)::bit(4))::bit(8)::int);
  -- Set variant bits to 10 (RFC 4122)
  uuid_bytes = set_byte(uuid_bytes, 8, (b'10' || get_byte(uuid_bytes, 8)::bit(6))::bit(8)::int);
  RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;

-- Switch all primary key defaults from v4 to v7
ALTER TABLE users                  ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE workspaces             ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE workspace_memberships  ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE credentials            ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE membership_credentials ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE access_schedules       ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE gate_access_codes      ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE gates                  ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE custom_domains         ALTER COLUMN id SET DEFAULT uuid_generate_v7();
