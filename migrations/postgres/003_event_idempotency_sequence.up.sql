ALTER TABLE analytics_events
    ADD COLUMN IF NOT EXISTS logical_event_id TEXT,
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
    ADD COLUMN IF NOT EXISTS tab_id TEXT,
    ADD COLUMN IF NOT EXISTS sequence BIGINT,
    ADD COLUMN IF NOT EXISTS previous_logical_event_id TEXT,
    ADD COLUMN IF NOT EXISTS dedup_strategy TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS ux_analytics_events_logical_event_id_present
ON analytics_events(tenant_id, site_id, logical_event_id)
WHERE logical_event_id IS NOT NULL AND logical_event_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS ux_analytics_events_idempotency_key_present
ON analytics_events(tenant_id, site_id, idempotency_key)
WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

CREATE INDEX IF NOT EXISTS ix_analytics_events_session_tab_sequence
ON analytics_events(tenant_id, site_id, session_id, tab_id, sequence);
