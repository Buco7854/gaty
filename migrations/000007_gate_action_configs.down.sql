ALTER TABLE gates
    DROP COLUMN IF EXISTS open_config,
    DROP COLUMN IF EXISTS close_config,
    DROP COLUMN IF EXISTS status_config;
