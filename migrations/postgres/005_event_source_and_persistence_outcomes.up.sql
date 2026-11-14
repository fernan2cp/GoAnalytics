ALTER TABLE analytics_events
    ADD COLUMN event_source TEXT NOT NULL DEFAULT 'web';

ALTER TABLE analytics_events
    ADD CONSTRAINT ck_analytics_events_event_source
    CHECK (event_source IN ('web', 'server'));
