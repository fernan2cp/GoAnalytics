# System Tracking Contracts

## Estado

Referencia publica para integradores de GoAnalytics. Este documento consolida el contrato estable de system tracking v1 y evita que proyectos consumidores dependan del SDD interno.

GoAnalytics v1 mantiene responsabilidades acotadas:

- ingesta HTTP de eventos;
- validacion de token, contrato minimo y seguridad de payload;
- persistencia de eventos crudos en `analytics_events`;
- normalizacion existente de eventos de items soportados;
- SDK web con helpers genericos como azucar sobre `track`.

GoAnalytics v1 no implementa endpoint online de agregados/top items ni materializacion propia de sugerencias por contexto.

## Endpoint De Ingesta

```http
POST /v1/events
Authorization: Bearer <tracking_jwt>
Content-Type: application/json
```

El body puede ser un evento suelto o un batch:

```json
{
  "events": [
    {
      "event_id": "018f9b8e-0000-7000-a000-000000000001",
      "logical_event_id": "feature_opened:sess_1:tab_1:catalog_search",
      "idempotency_key": "open:catalog_search:flow_1",
      "tab_id": "tab_1",
      "sequence": 1,
      "previous_logical_event_id": null,
      "event_name": "feature_opened",
      "event_version": 1,
      "timestamp": "2026-05-05T12:00:00.000Z",
      "anonymous_id": "anon_1",
      "session_id": "sess_1",
      "user_id": null,
      "origin": "https://workspace.example.com",
      "url": "https://workspace.example.com/search",
      "path": "/search",
      "referrer": "",
      "properties": {
        "open_reason": "user_action"
      },
      "context": {
        "app_area": "backoffice",
        "feature": "catalog_search",
        "surface": "drawer",
        "entry_point": "navigation_menu",
        "component_id": "item_search",
        "flow_id": "flow_1"
      }
    }
  ]
}
```

Alias compatibles:

- `metadata` mapea a `properties`.
- `event_type` mapea a `event_name`.
- `page_url` mapea a `url`.

Respuesta aceptada para procesamiento:

```json
{
  "accepted": 1,
  "rejected": 0,
  "event_ids": ["018f9b8e-0000-7000-a000-000000000001"]
}
```

## Campos Comunes

Campos recomendados por evento:

- `event_id`: identificador tecnico unico del intento de evento.
- `logical_event_id`: identidad logica estable para deduplicacion funcional.
- `idempotency_key`: clave funcional del dominio cuando existe.
- `tab_id`, `sequence`, `previous_logical_event_id`: reconstruccion de recorrido en una pestana.
- `event_name`: nombre en `snake_case`.
- `event_version`: version semantica del evento; empieza en `1`.
- `timestamp`: UTC RFC3339.
- `anonymous_id` o `session_id`: al menos uno debe estar presente.
- `user_id`: opcional, nunca debe contener secretos.
- `origin`, `url`, `path`, `referrer`: contexto de navegacion.
- `properties`: datos especificos del hecho observado.
- `context`: entorno funcional, tecnico o de ubicacion logica.

## Context

`context` es generico, opcional y sin catalogo cerrado. Los campos desconocidos son validos si son JSON seguro y cumplen limites.

Campos recomendados:

- `app_area`
- `feature`
- `surface`
- `entry_point`
- `flow_id`
- `component_id`
- `entity_type`
- `entity_id`
- `runtime`

Regla de frontera:

- `context` describe donde y bajo que entorno ocurre el evento.
- `properties` describe que ocurrio y con que datos especificos.

Ejemplo de busqueda:

```json
{
  "event_name": "search_result_selected",
  "properties": {
    "search_id": "search_1",
    "result_type": "item",
    "result_id": "item_100",
    "position": 1,
    "query_length": 4
  },
  "context": {
    "app_area": "backoffice",
    "feature": "catalog_search",
    "surface": "embedded_panel",
    "component_id": "item_search"
  }
}
```

## Eventos Genericos V1

Eventos de uso:

- `feature_opened`
- `feature_action_performed`

Eventos de busqueda:

- `search_performed`
- `search_result_selected`
- `search_empty_result`
- `search_abandoned`

Eventos de formularios:

- `form_validation_attempt`
- `form_step_viewed`
- `form_step_advanced`
- `form_completed`
- `form_abandoned`

Eventos de frustracion y flujo:

- `rage_click_detected`
- `dead_click_detected`
- `flow_abandoned`

Evento de seleccion contextual:

- `item_selected_for_context`

`item_selected_for_context` se persiste como evento crudo en `analytics_events` en v1. No crea fila en `analytics_event_items` salvo que una extension futura lo documente explicitamente.

## Eventos De Items Normalizados

GoAnalytics mantiene normalizacion de items para estos eventos:

- `item_impression`
- `item_viewed`
- `item_image_zoomed`
- `cart_item_added`
- `checkout_started`
- `purchase_completed`

Estos eventos pueden generar filas en `analytics_event_items`; `checkout_started` y `purchase_completed` tambien pueden generar cabecera en `analytics_event_orders`.

Campos principales en `properties`:

- `item_id`, `variant_id`, `sku`, `item_type`, `item_class_id`, `category_ids`.
- `surface`, `position`, `page`, `search_term`.
- `ranking_run_id`, `ranking_version`, `list_instance_id`, `impression_batch_id`.
- `visible_ratio`, `visible_time_ms`, `viewport_width`, `viewport_height`, `rendered_at`.
- `cart_id`, `checkout_id`, `order_id`, `order_line_id`.
- `quantity`, `unit_price`, `currency`, importes y costos opcionales.

## Seguridad Y Limites

Claves bloqueadas en `properties` y `context`, incluyendo objetos o arrays anidados:

```text
password
token
access_token
refresh_token
authorization
cookie
secret
private_key
credit_card
card_number
cvv
dni
document
```

Limites del worker:

- `properties` serializado: maximo 64 KiB.
- `context` serializado: maximo 64 KiB.
- profundidad maxima por objeto: 16.

`search_term` es opt-in. Si puede contener texto libre sensible, no enviarlo; usar `query_length`, `has_query`, `filters_count`, `result_count` y `search_id`.

Los eventos de formularios no deben transportar valores ingresados por usuarios. Solo nombres tecnicos de campos, codigos de error y conteos.

## Deduplicacion

Recomendaciones:

- Usar `event_id` unico por intento tecnico.
- Usar `logical_event_id` para eventos logicos que pueden repetirse por reintentos, doble montaje o doble handler.
- Usar `idempotency_key` para operaciones funcionales del dominio.
- Mantener `tab_id` y `sequence` cuando el SDK o aplicacion puede generarlos.

El SDK web genera estos valores automaticamente cuando el consumidor no los provee.

## SDK Web

Helpers v1 disponibles:

- `track(eventName, properties?, options?)`
- `page(properties?, options?)`
- `checkoutStarted(payload, options?)`
- `featureOpened(payload?, options?)`
- `featureActionPerformed(payload, options?)`
- `searchPerformed(payload, options?)`
- `searchResultSelected(payload, options?)`
- `searchEmptyResult(payload, options?)`
- `searchAbandoned(payload, options?)`
- `rageClickDetected(payload?, options?)`
- `deadClickDetected(payload?, options?)`
- `flowAbandoned(payload?, options?)`
- `formAttempt(payload, options?)`
- `formCompleted(payload, options?)`
- `formAbandoned(payload, options?)`
- `formStepAdvanced(payload, options?)`
- `formStepViewed(payload, options?)`

Todos delegan en `track`; `TrackOptions.context`, `logicalEventId` e `idempotencyKey` siguen disponibles.

## Persistencia

Eventos validos se guardan en `analytics_events` con:

- `properties` JSONB no nulo.
- `context` JSONB no nulo.

Eventos rechazados por contrato, seguridad, dominio o deduplicacion se auditan en la tabla de rechazos configurada por el worker.

## Agregados Y Top Items

Decision v1: GoAnalytics no expone `GET /v1/aggregates/items/top`, no crea `analytics_context_item_aggregates` y no ejecuta scoring contextual online.

Integradores que necesiten top items por contexto deben leer eventos normalizados en modo server-to-server o export y materializar agregados propios. La lectura debe separar tenants/sites, no usar tracking JWT publico y no exponer datos personales.

Una futura version puede agregar agregados genericos si define migracion, job fuera de ingesta, algoritmo versionado, credencial de lectura y pruebas de seguridad/carga.