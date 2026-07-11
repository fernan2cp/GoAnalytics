# Plan De Pruebas

## Pruebas Unitarias De Extraccion

Cubrir:

- item unico en `properties`;
- lista de items en `items`;
- datos equivalentes en `metadata`;
- variantes con `variant_id` y `sku`;
- importes, descuentos y costos opcionales;
- evento sin items.

## Pruebas De `item_impression`

Validar:

- impresion con `visible_ratio >= 50` y `visible_time_ms >= 1000`;
- impresion incompleta marcada en `metadata`;
- ausencia de emision por skeleton o placeholder representada como payload invalido para scoring;
- presencia de `list_instance_id` e `impression_batch_id`.

## Pruebas De Deduplicacion

Cubrir:

- deduplicacion exacta por `event_id`;
- deduplicacion por `idempotency_key`;
- deduplicacion por `logical_event_id`;
- deduplicacion de impresion por `session_id`, `surface`, `list_instance_id`, `item_id` y `variant_id`;
- deduplicacion estricta de compra por `order_id` y `order_line_id`.

## Pruebas De Persistencia

Validar con repositorio PostgreSQL:

- insercion de evento base sin items;
- insercion de evento base con un item;
- insercion de evento base con multiples items;
- obtencion y uso de `analytics_events.id` como `analytics_event_id`;
- rollback logico si falla la insercion de detalles;
- indices creados por migracion `up`;
- eliminacion correcta por migracion `down`.

## Pruebas De Contrato

Validar que la API acepte:

- batch mixto con eventos con y sin items;
- `item_type` con valores `product`, `service` y `subscription_plan`;
- campos economicos opcionales;
- campos de orden y linea;
- campos tecnicos de impresion.

Tambien validar que no se requiera que todos los eventos tengan item.

## Casos Borde

- `purchase_completed` sin `order_line_id`: persiste, pero queda incompleto para scoring.
- `item_impression` sin `list_instance_id`: persiste, pero queda incompleto para scoring.
- item con `variant_id` vacio: se trata como item canonico.
- importes ausentes: no bloquean interacciones no economicas.
- `category_ids` vacio: no bloquea persistencia.

## Criterios De Aceptacion

- El paquete de eventos mantiene compatibilidad con el contrato actual.
- No se duplican eventos base.
- No se pierden detalles de items cuando el evento los contiene.
- Las compras duplicadas no duplican lineas para scoring.
- Las impresiones tienen evidencia minima de visibilidad.
- Go Analytics no calcula ni persiste scores.
