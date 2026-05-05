CREATE TABLE IF NOT EXISTS analytics_rejected_events (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT,
    site_public_id TEXT,
    env TEXT,
    reason TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'warning',
    origin TEXT,
    url TEXT,
    ip_hash TEXT,
    user_agent TEXT,
    raw_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_analytics_rejected_events_site_time
ON analytics_rejected_events(site_public_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_rejected_events_reason
ON analytics_rejected_events(reason);
