-- Per-action integration configs for gates
-- Replaces the single integration_type/integration_config with three independent configs.

ALTER TABLE gates
    ADD COLUMN open_config   JSONB,
    ADD COLUMN close_config  JSONB,
    ADD COLUMN status_config JSONB;

-- Migrate existing MQTT gates: open and status use MQTT by default
UPDATE gates
SET open_config   = '{"type": "MQTT"}'::jsonb,
    status_config = '{"type": "MQTT"}'::jsonb
WHERE integration_type = 'MQTT';
