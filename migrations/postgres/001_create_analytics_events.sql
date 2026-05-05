CREATE TABLE IF NOT EXISTS analytics_events (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL,
    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    site_code TEXT NOT NULL,
    env TEXT NOT NULL DEFAULT 'production',
    event_name TEXT NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,
    event_time TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    anonymous_id TEXT,
    user_id TEXT,
    session_id TEXT,
    origin TEXT,
    url TEXT,
    path TEXT,
    referrer TEXT,
    user_agent TEXT,
    ip_hash TEXT,
    sdk_name TEXT,
    sdk_version TEXT,
    jwt_id TEXT,
    token_version INTEGER,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    context JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_analytics_events_event_id
ON analytics_events(event_id);

CREATE INDEX IF NOT EXISTS ix_analytics_events_tenant_site_time
ON analytics_events(tenant_id, site_id, event_time DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_events_name_time
ON analytics_events(event_name, event_time DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_events_session
ON analytics_events(session_id);

CREATE INDEX IF NOT EXISTS ix_analytics_events_user
ON analytics_events(user_id);
