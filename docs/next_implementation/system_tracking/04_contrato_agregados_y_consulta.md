# Contrato De Agregados Y Consulta

## Objetivo

Documentar una extension opcional para que GoAnalytics pueda exponer agregados genericos, por ejemplo top items por contexto, sin asumir reglas de negocio de un integrador.

Esta extension no debe estar en el path critico de ingesta.

## Modelo Conceptual

Un agregado contextual resume eventos por:

```text
tenant_id + site_id/site_code + app_area + feature + surface + entry_point + component_id + entity_type + item_type + item_id
```

No todos los campos son requeridos. La consulta debe aceptar filtros parciales y usar fallback definido por el consumidor o por una politica generica.

## Caso Top Items

Top items por contexto permite responder:

- Que items se seleccionan mas en una superficie.
- Que items generan mas conversion o valor.
- Que sugerencias conviene mostrar antes de una busqueda manual.

Eventos fuente sugeridos:

- `purchase_completed`
- `cart_item_added`
- `item_selected_for_context`
- `search_result_selected`
- `checkout_started`
- `item_viewed`
- `item_impression`

## Tabla O Vista Opcional

Nombre sugerido si se implementa en GoAnalytics:

```text
analytics_context_item_aggregates
```

Campos sugeridos:

- `id`
- `tenant_id`
- `site_id`
- `site_code`
- `app_area`
- `feature`
- `surface`
- `entry_point`
- `component_id`
- `entity_type`
- `item_type`
- `item_id`
- `variant_id`
- `score`
- `event_count`
- `weighted_event_count`
- `last_event_time`
- `window_start`
- `window_end`
- `algorithm_version`
- `metadata`
- `computed_at`

Todos los campos de contexto deben ser nullable para permitir agregados parciales.

## Endpoint Opcional

Si GoAnalytics expone consulta HTTP, el contrato debe ser generico:

```http
GET /v1/aggregates/items/top
Authorization: Bearer <query_jwt>
```

Query params:

- `site_code`
- `app_area`
- `feature`
- `surface`
- `entry_point`
- `component_id`
- `entity_type`
- `item_type`
- `limit`
- `window_days`
- `fallback`, booleano opcional

Respuesta:

```json
{
  "context": {
    "site_code": "site_a",
    "app_area": "backoffice",
    "feature": "sales",
    "surface": "drawer",
    "entry_point": "detail",
    "component_id": "item_search",
    "item_type": "service"
  },
  "items": [
    {
      "item_id": "123",
      "variant_id": null,
      "item_type": "service",
      "score": 184.5,
      "event_count": 32,
      "last_event_time": "2026-06-27T10:00:00Z"
    }
  ],
  "fallback_level": "exact_context",
  "window_start": "2026-03-29T00:00:00Z",
  "window_end": "2026-06-27T00:00:00Z",
  "computed_at": "2026-06-27T10:05:00Z",
  "algorithm_version": "context_items_v1"
}
```

## Seguridad Del Endpoint

La consulta de agregados no debe reutilizar necesariamente el tracking JWT de ingesta. Se recomienda un `query_jwt` o credencial server-to-server con permisos de lectura.

El endpoint no debe devolver datos personales. Solo IDs tecnicos y metricas agregadas.

## Fallback

GoAnalytics puede exponer el `fallback_level`, pero la politica exacta puede ser del integrador.

Fallback generico sugerido:

1. `exact_context`
2. `without_entry_point`
3. `feature_component`
4. `app_area_item_type`
5. `site_item_type`
6. `site_global`

## Alternativa Sin Endpoint

Si no se implementa endpoint de consulta, GoAnalytics puede limitarse a persistir eventos y detalles normalizados. El integrador puede leer la base de analytics mediante jobs server-to-server y materializar sus propios agregados.

Esta alternativa conserva mejor la separacion de responsabilidades y ya es compatible con integradores que tienen pipelines propios.
