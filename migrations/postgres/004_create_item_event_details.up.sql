CREATE TABLE IF NOT EXISTS analytics_event_items (
    id BIGSERIAL PRIMARY KEY,
    analytics_event_id BIGINT NOT NULL REFERENCES analytics_events(id) ON DELETE CASCADE,
    client_event_id TEXT NOT NULL,
    logical_event_id TEXT,
    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    site_code TEXT NOT NULL,
    event_name TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    anonymous_id TEXT,
    session_id TEXT,
    user_id TEXT,
    item_id TEXT NOT NULL,
    variant_id TEXT,
    sku TEXT,
    item_type TEXT,
    item_class_id TEXT,
    category_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    surface TEXT,
    position INTEGER,
    page INTEGER,
    search_term TEXT,
    ranking_run_id TEXT,
    ranking_version TEXT,
    list_instance_id TEXT,
    impression_batch_id TEXT,
    visible_ratio NUMERIC(8, 4),
    visible_time_ms BIGINT,
    viewport_width INTEGER,
    viewport_height INTEGER,
    rendered_at TIMESTAMPTZ,
    cart_id TEXT,
    checkout_id TEXT,
    order_id TEXT,
    order_line_id TEXT,
    quantity NUMERIC(18, 6),
    unit_price NUMERIC(18, 4),
    currency TEXT,
    gross_amount NUMERIC(18, 4),
    net_amount NUMERIC(18, 4),
    discount_amount NUMERIC(18, 4),
    unit_cost NUMERIC(18, 4),
    cost_amount NUMERIC(18, 4),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS analytics_event_orders (
    id BIGSERIAL PRIMARY KEY,
    analytics_event_id BIGINT NOT NULL REFERENCES analytics_events(id) ON DELETE CASCADE,
    client_event_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    site_code TEXT NOT NULL,
    event_name TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    cart_id TEXT,
    checkout_id TEXT,
    order_id TEXT,
    currency TEXT,
    subtotal_amount NUMERIC(18, 4),
    discount_amount NUMERIC(18, 4),
    shipping_amount NUMERIC(18, 4),
    tax_amount NUMERIC(18, 4),
    gross_amount NUMERIC(18, 4),
    net_amount NUMERIC(18, 4),
    cost_amount NUMERIC(18, 4),
    payment_method_id TEXT,
    payment_provider TEXT,
    shipping_method_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_event
ON analytics_event_items(analytics_event_id);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_item_time
ON analytics_event_items(tenant_id, site_id, item_id, event_time DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_name_time
ON analytics_event_items(tenant_id, site_id, event_name, event_time DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_surface_time
ON analytics_event_items(tenant_id, site_id, surface, event_time DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_order_line
ON analytics_event_items(tenant_id, site_id, order_id, order_line_id);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_ranking_run
ON analytics_event_items(ranking_run_id);

CREATE INDEX IF NOT EXISTS ix_analytics_event_items_category_ids_gin
ON analytics_event_items USING GIN(category_ids);

CREATE INDEX IF NOT EXISTS ix_analytics_event_orders_order
ON analytics_event_orders(tenant_id, site_id, order_id);

CREATE INDEX IF NOT EXISTS ix_analytics_event_orders_time
ON analytics_event_orders(tenant_id, site_id, event_time DESC);
