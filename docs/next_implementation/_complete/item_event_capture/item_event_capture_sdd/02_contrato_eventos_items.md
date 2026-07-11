# Contrato De Eventos De Items

## Eventos Soportados

| Evento | Uso |
|---|---|
| `item_impression` | Item visible en una superficie. |
| `item_viewed` | Vista de detalle de item. |
| `item_image_zoomed` | Interaccion fuerte con imagen del item. |
| `cart_item_added` | Item agregado al carrito. |
| `checkout_started` | Inicio de checkout con items. |
| `purchase_completed` | Compra confirmada con items. |

Los nombres son estables y deben enviarse en `snake_case`.

## Forma Del Payload

El contrato publico puede recibir datos de item en `properties`, `metadata` o `items`. La implementacion debe normalizar cualquiera de estas formas hacia una representacion interna comun.

Para eventos con un solo item, se acepta un objeto con los campos de item. Para eventos multi-item, se debe enviar una lista `items`.

## Campos Comunes De Item

Campos esperados cuando apliquen:

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

## Campos De Impresion

`item_impression` debe incluir evidencia de visibilidad:

- `visible_ratio`
- `visible_time_ms`
- `viewport_width`
- `viewport_height`
- `list_instance_id`
- `impression_batch_id`
- `rendered_at`

Una impresion valida cumple:

```text
visible_ratio >= 50%
AND visible_time_ms >= 1000
AND document.visibilityState == "visible"
AND item real renderizado, no skeleton ni placeholder
```

No debe emitirse por render tecnico, precarga, skeleton, item fuera del viewport, pestana en segundo plano o re-renderizado.

## Campos De Carrito, Checkout Y Compra

Eventos de carrito, checkout y compra deben usar, cuando existan:

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

`unit_cost` y `cost_amount` son snapshots opcionales. No deben recalcularse desde valores actuales del item.

## Reglas Por Evento

| Evento | Reglas minimas |
|---|---|
| `item_impression` | Requiere `item_id`, `surface`, `list_instance_id`, `visible_ratio`, `visible_time_ms`. |
| `item_viewed` | Requiere `item_id`; recomienda `surface` y `category_ids`. |
| `item_image_zoomed` | Requiere `item_id`; recomienda `variant_id` si la imagen pertenece a una variante. |
| `cart_item_added` | Requiere `item_id`, `cart_id`, `quantity`; recomienda importes. |
| `checkout_started` | Requiere `checkout_id` o `cart_id`; recomienda lista `items`. |
| `purchase_completed` | Requiere `order_id`; para scoring confiable requiere `order_line_id` por item. |

Si falta un campo critico para scoring, el evento puede persistirse para auditoria, pero el detalle normalizado debe marcarse como incompleto mediante `metadata`.
