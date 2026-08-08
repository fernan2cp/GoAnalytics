-- ADVERTENCIA: este down elimina event_source y su trazabilidad. Solo debe
-- ejecutarse en desarrollo o con autorización explícita de pérdida de datos.
ALTER TABLE analytics_events
    DROP CONSTRAINT IF EXISTS ck_analytics_events_event_source;

ALTER TABLE analytics_events
    DROP COLUMN IF EXISTS event_source;
