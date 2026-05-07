DROP INDEX IF EXISTS ix_analytics_events_session_tab_sequence;
DROP INDEX IF EXISTS ux_analytics_events_idempotency_key_present;
DROP INDEX IF EXISTS ux_analytics_events_logical_event_id_present;

ALTER TABLE analytics_events
    DROP COLUMN IF EXISTS dedup_strategy,
    DROP COLUMN IF EXISTS previous_logical_event_id,
    DROP COLUMN IF EXISTS sequence,
    DROP COLUMN IF EXISTS tab_id,
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS logical_event_id;
