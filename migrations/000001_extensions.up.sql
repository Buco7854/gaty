CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- UUID v7: timestamp-ordered UUID (ms precision prefix + 62 random bits).
-- Used as DEFAULT for all primary keys instead of gen_random_uuid() (v4).
-- Benefits over v4: monotone B-tree insertions, less index fragmentation,
-- natural sort order by creation time. Conflict risk unchanged (62 random bits).
CREATE OR REPLACE FUNCTION uuid_generate_v7() RETURNS uuid
LANGUAGE plpgsql PARALLEL SAFE
AS $$
DECLARE
  unix_ts_ms bytea;
  uuid_bytes bytea;
BEGIN
  unix_ts_ms = substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3);
  uuid_bytes = uuid_send(gen_random_uuid());
  uuid_bytes = overlay(uuid_bytes PLACING unix_ts_ms FROM 1 FOR 6);
  uuid_bytes = set_byte(uuid_bytes, 6, (b'0111' || get_byte(uuid_bytes, 6)::bit(4))::bit(8)::int);
  uuid_bytes = set_byte(uuid_bytes, 8, (b'10'   || get_byte(uuid_bytes, 8)::bit(6))::bit(8)::int);
  RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;
