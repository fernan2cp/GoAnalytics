# Diseno Tecnico

## Enfoque General

La evolucion se implementa como una extension compatible del contrato actual. La v1 debe priorizar aceptacion segura, persistencia cruda, helpers SDK y documentacion publica. No requiere migracion obligatoria porque `analytics_events` ya tiene `properties` y `context` JSONB.

Los agregados contextuales se tratan como fase opt-in. Solo se implementan si existe una decision explicita de producto u operacion, con job fuera de ingesta y credencial de lectura separada.

## Flujo De Ingesta

1. El integrador envia eventos por el contrato actual, con `context` opcional.
2. El mapper HTTP conserva alias existentes y entrega `dto.IngestEvent` con `Properties` y `Context`.
3. `IngestEventsUseCase` valida token, rate limit y campos minimos, normaliza mapas nulos y publica el evento crudo.
4. El worker consume el evento, bloquea claves sensibles en `properties` y `context`, valida site y deduplica.
5. El repositorio persiste `analytics_events` con `properties` y `context` completos.
6. La normalizacion de items existente sigue creando filas en `analytics_event_items` y `analytics_event_orders` cuando corresponde.

## Contrato De Contexto

`context` debe aceptar campos recomendados sin convertirlos en obligatorios:

- `app_area`
- `feature`
- `surface`
- `entry_point`
- `component_id`
- `flow_id`
- `entity_type`
- `entity_id`
- `runtime`

La validacion debe controlar forma JSON, claves sensibles y limites. No debe rechazar valores desconocidos por no pertenecer a un catalogo.

`properties` conserva payload especifico del evento. Para busqueda, por ejemplo, `search_id`, `query_length`, `filters` y `results_count` pertenecen a `properties`; `feature`, `surface` y `component_id` pertenecen a `context`.

## Eventos Genericos

Los eventos genericos se aceptan por el mismo `track` existente. No requieren tablas propias para v1:

- Uso: `feature_opened`, `feature_action_performed`.
- Busqueda: `search_performed`, `search_result_selected`, `search_empty_result`, `search_abandoned`.
- Formularios: `form_validation_attempt`, `form_completed`, `form_abandoned`, `form_step_advanced`, `form_step_viewed`.
- Frustracion: `rage_click_detected`, `dead_click_detected`, `flow_abandoned`.
- Seleccion contextual: `item_selected_for_context`.

Los tests deben verificar payload minimo, contexto opcional, persistencia cruda y ausencia de campos de integrador especifico.

## SDK Web

El SDK debe mantener `track` como primitiva. Los helpers nuevos deben construir `properties` tipadas, aceptar `TrackOptions` y delegar a `track`.

Helpers candidatos para v1:

- `featureOpened(payload, options)`
- `featureActionPerformed(payload, options)`
- `searchPerformed(payload, options)`
- `searchResultSelected(payload, options)`
- `searchEmptyResult(payload, options)`
- `searchAbandoned(payload, options)`
- `rageClickDetected(payload, options)`
- `deadClickDetected(payload, options)`
- `flowAbandoned(payload, options)`

Los helpers no deben ocultar `logicalEventId`, `idempotencyKey` ni `context`; esos campos siguen entrando por `TrackOptions`.

## Seguridad

El worker ya bloquea claves sensibles anidadas. La implementacion debe reforzar tests y documentar la regla. Si se agregan limites adicionales, deben vivir en dominio o aplicacion, no en adaptadores concretos, para mantener la arquitectura hexagonal.

El SDK ya sanitiza payloads de formularios. La implementacion debe mantener esa proteccion y evitar que helpers de formulario acepten valores libres. Para busqueda, `search_term` debe documentarse como opt-in seguro; si hay duda, usar `query_length`.

## Persistencia

La persistencia base no requiere migracion:

- `analytics_events.properties` guarda el hecho observado.
- `analytics_events.context` guarda contexto funcional y tecnico.

No agregar indices JSONB en v1 salvo evidencia de consulta frecuente. Si se decide normalizar contexto en `analytics_event_items`, usar campos nullable y evitar conflicto con `surface` de item. La opcion preferida inicial es leer contexto desde `analytics_events` al agregar metricas offline.

## Agregados Contextuales Opcionales

Si se implementa `analytics_context_item_aggregates`, debe ser por job batch, worker periodico, vista materializada o export. La tabla o vista debe agrupar por tenant, site y campos contextuales nullable.

El endpoint opcional `GET /v1/aggregates/items/top` debe:

- usar credencial de lectura, no tracking JWT publico;
- aceptar filtros parciales;
- respetar `limit` y ventana temporal;
- devolver `fallback_level`, `algorithm_version`, `computed_at` y metricas agregadas;
- no mezclar tenants ni sites.

## Observabilidad

La v1 debe conservar logs de rechazo por payload invalido o clave bloqueada. Si se agregan agregados, incluir logs de job, conteo de filas procesadas, ventana, duracion, algoritmo y errores por tenant/site.

## Referencia Para Integradores

El ultimo paso de implementacion debe crear `docs/integration/system-tracking-contracts.md` como documento de referencia para proyectos que integren este microservicio.

Ese documento debe consolidar:

- endpoint de ingesta, autenticacion y forma batch;
- campos comunes obligatorios y opcionales;
- contrato de `context` y relacion con `properties`;
- catalogo de eventos genericos creados o soportados;
- ejemplos de payload minimo y recomendado por evento;
- reglas de seguridad y claves bloqueadas;
- criterios de deduplicacion con `logical_event_id` e `idempotency_key`;
- comportamiento esperado de aceptacion, rechazo y persistencia;
- contrato de agregados si se implementa, o estado explicito de fase posterior si no se implementa.

## Rollout Y Rollback

La fase de contrato, SDK y documentacion es compatible hacia atras. Rollback: quitar helpers SDK nuevos y conservar `track` manual, sin migraciones.

La fase de agregados requiere rollout separado. Rollback: detener job, ocultar endpoint y conservar eventos crudos. Si hubo migracion de tabla de agregados, debe tener `down` simetrico y no afectar `analytics_events`.
