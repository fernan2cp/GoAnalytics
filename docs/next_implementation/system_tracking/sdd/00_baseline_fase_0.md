# Baseline Fase 0

## Estado Actual Confirmado

`GoAnalytics` ya tiene una arquitectura separada por `services/ingest`, `services/worker`, `packages/web-sdk`, migraciones PostgreSQL y documentacion publica.

La ingesta HTTP en `services/ingest/internal/adapters/inbound/http/request_mapper.go` ya acepta `properties`, `metadata` y `context`. Tambien conserva compatibilidad con payload batch y single-event, mapea `metadata` hacia `properties`, `event_type` hacia `event_name` y `page_url` hacia `url`.

El caso de uso `services/ingest/internal/application/usecases/ingest_events.go` valida token, rate limit, batch size y campos minimos antes de publicar eventos crudos. No valida catalogos de negocio.

El dominio de ingesta en `services/ingest/internal/domain/event/validation.go` exige campos minimos como `event_id`, `event_name`, `event_version`, `timestamp`, identidad anonima o sesion, `origin`, `url` y `path`. `NormalizeMap` garantiza mapas no nulos para `properties` y `context`.

El worker en `services/worker/internal/application/usecases/process_events.go` valida metadata de site, deduplica, bloquea claves sensibles, persiste eventos validos y registra rechazos.

La seguridad de payload en `services/worker/internal/application/usecases/payload_safety.go` ya inspecciona claves sensibles en `properties` y `context`, incluyendo objetos anidados. La lista bloquea, entre otras, `password`, `token`, `access_token`, `refresh_token`, `authorization`, `cookie`, `secret`, `private_key`, `credit_card`, `card_number`, `cvv`, `dni` y `document`.

La migracion `migrations/postgres/001_create_analytics_events.up.sql` ya crea `analytics_events` con `properties JSONB NOT NULL DEFAULT '{}'::jsonb` y `context JSONB NOT NULL DEFAULT '{}'::jsonb`.

El repositorio PostgreSQL `services/worker/internal/adapters/outbound/postgres/event_repository.go` persiste `properties` y `context` como JSONB y usa `ON CONFLICT (event_id)` para idempotencia del evento base.

La migracion `migrations/postgres/004_create_item_event_details.up.sql` ya crea `analytics_event_items` y `analytics_event_orders`. `analytics_event_items` contiene `surface`, `search_term`, `position`, `item_type`, `item_id`, datos de visibilidad, carrito, checkout y orden.

La normalizacion de items en `services/worker/internal/application/usecases/item_details.go` extrae items desde `properties.items`, `properties.item` o campos directos con `item_id`. Todavia no copia campos contextuales como `app_area`, `feature`, `entry_point` o `component_id` a `analytics_event_items`.

El SDK web en `packages/web-sdk/src/index.ts` ya expone `TrackOptions.context`, agrega `sdk_name` y `sdk_version` al `context`, y tiene helpers para `checkoutStarted`, eventos de formulario y `page`. No hay helpers dedicados para `feature_opened`, `feature_action_performed`, busqueda o eventos de frustracion.

La documentacion publica en `docs/event-contract.md`, `docs/event-conventions.md` y `packages/web-sdk/README.md` ya cubre contrato base, convenciones de `properties` y `context`, claves sensibles y eventos de items. Falta incorporar el contrato generico de system tracking.

## Decisiones Cerradas

- Los campos nuevos deben ser opcionales.
- `context` describe entorno funcional y tecnico; `properties` describe el hecho observado.
- `GoAnalytics` no debe validar catalogos cerrados de valores para `app_area`, `feature`, `surface`, `entry_point` ni campos equivalentes.
- Los eventos de comportamiento son genericos y pueden coexistir con eventos especificos del integrador.
- Los helpers SDK deben ser azucar sobre `track`, no un transporte nuevo.
- La persistencia cruda en `analytics_events` es suficiente para v1 de contexto generico.
- Cualquier agregado contextual debe ejecutarse fuera del path critico de ingesta.
- Cualquier endpoint de consulta de agregados debe usar credencial de lectura server-to-server, no tracking JWT publico.

## Gaps A Validar Antes De Implementar

- Confirmar si la validacion actual de payload tiene limites suficientes de profundidad, cantidad de claves y tamanio por evento ademas del limite de batch/payload del SDK.
- Confirmar si la lista de claves sensibles del SDK y del worker debe unificarse para evitar diferencias entre cliente y backend.
- Confirmar si `search_term` se permitira en v1 o si la documentacion debe recomendar solo `query_length` salvo opt-in explicito del integrador.
- Confirmar si `item_selected_for_context` debe normalizarse en `analytics_event_items` en v1 o quedar solo en `analytics_events`.
- Confirmar si los agregados contextuales se implementaran dentro de `GoAnalytics` o quedaran para jobs externos del integrador.

## Riesgos

- Aceptar contexto arbitrario sin limites claros puede aumentar costo de almacenamiento y procesamiento.
- Documentar ejemplos con datos libres puede incentivar envio accidental de informacion personal.
- Duplicar semantica de `surface` entre items y contexto puede crear consultas ambiguas.
- Agregar indices JSONB sin evidencia puede degradar escritura o aumentar costo operativo.
- Exponer agregados con credenciales de ingesta podria filtrar metricas internas.
