CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Alias around the PostgreSQL 18 built-in uuidv7().
-- Used as DEFAULT for all primary keys (timestamp-ordered, compact B-tree index).
CREATE OR REPLACE FUNCTION uuid_generate_v7() RETURNS uuid
LANGUAGE sql PARALLEL SAFE
AS $$ SELECT uuidv7(); $$;
