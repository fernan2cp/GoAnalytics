# Modelo De Datos

## Tabla Principal Existente

`analytics_events` conserva el evento completo y sigue siendo la fuente de auditoria.

Identidad vigente:

```text
analytics_events.id = PK interna
analytics_events.event_id = ID enviado por el SDK
analytics_events.logical_event_id = ID logico del flujo
```

Las tablas nuevas no deben usar `event_id` para referirse a la PK interna. Deben usar `analytics_event_id`.

## Tabla `analytics_event_items`

`analytics_event_items` es una tabla 1:N denormalizada para scoring por item.

Campos requeridos para v1:

- `id`
- `analytics_event_id`
- `client_event_id`
- `logical_event_id`
- `tenant_id`
- `site_id`
- `site_code`
- `event_name`
- `event_time`
- `received_at`
- `anonymous_id`
- `session_id`
- `user_id`
- `item_id`
- `variant_id`
- `sku`
- `item_type`, con valores como `product`, `service` o `subscription_plan`
- `item_class_id`
- `category_ids`
- `surface`
- `position`
- `page`
- `search_term`
- `ranking_run_id`
- `ranking_version`
- `list_instance_id`
- `impression_batch_id`
- `visible_ratio`
- `visible_time_ms`
- `viewport_width`
- `viewport_height`
- `rendered_at`
- `cart_id`
- `checkout_id`
- `order_id`
- `order_line_id`
- `quantity`
- `unit_price`
- `currency`
- `gross_amount`
- `net_amount`
- `discount_amount`
- `unit_cost`
- `cost_amount`
- `metadata`
- `created_at`

## Tipos Sugeridos

- Identificadores externos: `TEXT`.
- Importes y costos: `NUMERIC`.
- Cantidades: `NUMERIC` para permitir cantidades no enteras si una integracion lo requiere.
- `category_ids`: `TEXT[]` o `JSONB`; si se usa PostgreSQL, preferir una forma compatible con indice GIN.
- `metadata`: `JSONB`.
- Fechas: `TIMESTAMPTZ`.

## Tabla `analytics_event_orders`

`analytics_event_orders` es recomendable y no bloqueante para la primera implementacion. Si entra en v1, debe guardar cabecera de checkout u orden.

Campos sugeridos:

- `id`
- `analytics_event_id`
- `client_event_id`
- `tenant_id`
- `site_id`
- `site_code`
- `event_name`
- `event_time`
- `cart_id`
- `checkout_id`
- `order_id`
- `currency`
- `subtotal_amount`
- `discount_amount`
- `shipping_amount`
- `tax_amount`
- `gross_amount`
- `net_amount`
- `cost_amount`
- `payment_method_id`
- `payment_provider`
- `shipping_method_id`
- `metadata`
- `created_at`

## Indices

Indices minimos:

- `analytics_event_items(analytics_event_id)`.
- `analytics_event_items(tenant_id, site_id, item_id, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, event_name, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, surface, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, order_id, order_line_id)`.
- `analytics_event_items(ranking_run_id)`.
- `GIN(category_ids)` si `category_ids` se guarda como array o JSONB.

Indice recomendado para `analytics_event_orders`:

- `analytics_event_orders(tenant_id, site_id, order_id)`.
- `analytics_event_orders(tenant_id, site_id, event_time DESC)`.

## Integridad

`analytics_event_items.analytics_event_id` debe referenciar `analytics_events.id`. La persistencia debe ejecutarse de forma atomica para evitar eventos base sin detalles de item cuando el evento incluya items validos.
