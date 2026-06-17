DROP INDEX IF EXISTS ix_analytics_event_orders_time;
DROP INDEX IF EXISTS ix_analytics_event_orders_order;
DROP INDEX IF EXISTS ix_analytics_event_items_category_ids_gin;
DROP INDEX IF EXISTS ix_analytics_event_items_ranking_run;
DROP INDEX IF EXISTS ix_analytics_event_items_order_line;
DROP INDEX IF EXISTS ix_analytics_event_items_surface_time;
DROP INDEX IF EXISTS ix_analytics_event_items_name_time;
DROP INDEX IF EXISTS ix_analytics_event_items_item_time;
DROP INDEX IF EXISTS ix_analytics_event_items_event;

DROP TABLE IF EXISTS analytics_event_orders;
DROP TABLE IF EXISTS analytics_event_items;
