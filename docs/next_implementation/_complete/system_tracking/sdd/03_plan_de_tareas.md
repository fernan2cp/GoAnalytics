# Plan De Tareas

## Fase 1 - Contrato Publico Y Documentacion

### TASK-ST-0101: Actualizar contrato publico de contexto

Estado: `done`

Requisitos: `REQ-ST-001`, `REQ-ST-002`, `REQ-ST-003`, `REQ-ST-004`, `REQ-ST-026`, `REQ-ST-027`

Criterios: `AC-ST-001`, `AC-ST-002`, `AC-ST-011`

Acciones:

- Actualizar `docs/event-contract.md` con `context` generico opcional y ejemplos.
- Actualizar `docs/event-conventions.md` con separacion `properties` vs `context`.
- Documentar que los valores desconocidos son validos si son JSON seguro.
- Evitar ejemplos con dominios de integrador o datos sensibles.

Evidencia:

- 2026-06-27: `docs/event-contract.md` documenta `context` generico opcional, convenciones recomendadas y frontera `context` vs `properties`.
- 2026-06-27: `docs/event-conventions.md` acepta campos desconocidos seguros y define claves recomendadas sin catalogo cerrado.

### TASK-ST-0102: Documentar eventos genericos de comportamiento

Estado: `done`

Requisitos: `REQ-ST-005`, `REQ-ST-006`, `REQ-ST-007`, `REQ-ST-008`, `REQ-ST-009`, `REQ-ST-012`, `REQ-ST-026`, `REQ-ST-027`

Criterios: `AC-ST-003`, `AC-ST-004`, `AC-ST-011`

Acciones:

- Agregar ejemplos de `feature_opened` y `feature_action_performed`.
- Agregar ejemplos de eventos de busqueda y reglas de `search_term`.
- Reforzar eventos de formularios sin valores ingresados por usuarios.
- Agregar eventos de frustracion y `item_selected_for_context`.

Evidencia:

- 2026-06-27: `docs/event-contract.md` agrega eventos genericos de feature, busqueda, formularios, abandono y frustracion con ejemplos no acoplados.
- 2026-06-27: `docs/event-conventions.md` incorpora eventos recomendados, regla opt-in de `search_term` y mantiene formularios sin valores de usuario.

## Fase 2 - Validacion Y Seguridad Backend

### TASK-ST-0201: Cubrir seguridad de context en tests del worker

Estado: `done`

Requisitos: `REQ-ST-010`, `REQ-ST-012`, `REQ-ST-013`, `REQ-ST-028`

Criterios: `AC-ST-005`, `AC-ST-006`, `AC-ST-012`

Acciones:

- Agregar tests para claves sensibles en `context` de primer nivel.
- Agregar tests para claves sensibles anidadas en `context`.
- Verificar que campos desconocidos seguros en `context` no rechazan eventos.
- Mantener tests equivalentes para `properties`.

Evidencia:

- 2026-06-27: `services/worker/internal/application/usecases/payload_safety.go` inspecciona claves bloqueadas en `properties` y `context`, incluyendo mapas y arrays anidados.
- 2026-06-27: `services/worker/internal/application/usecases/process_events_test.go` cubre claves sensibles de primer nivel, anidadas y campos desconocidos seguros en `context`.
- 2026-06-27: Validacion ejecutada: `go test ./...` en `services/worker` y `services/ingest`.

### TASK-ST-0202: Definir y probar limites de payload contextual

Estado: `done`

Requisitos: `REQ-ST-011`, `REQ-ST-013`, `REQ-ST-028`

Criterios: `AC-ST-006`, `AC-ST-012`

Acciones:

- Revisar limites actuales de batch y payload entre SDK, ingesta y worker.
- Definir limite objetivo para tamanio, profundidad o cantidad de claves si falta cobertura backend.
- Implementar validacion en capa de dominio o aplicacion si corresponde.
- Agregar tests con contexto grande aceptado dentro del limite y rechazado fuera del limite.

Evidencia:

- 2026-06-27: Se definieron limites genericos de 64 KiB serializados y profundidad maxima 16 por objeto `properties`/`context` en el worker.
- 2026-06-27: `process_events.go` rechaza violaciones con `payload_too_large` y detalle auditable `payload_issue`.
- 2026-06-27: `docs/event-conventions.md` documenta los limites publicos de payload contextual.
- 2026-06-27: Validacion ejecutada: `go test ./...` en `services/worker` y `services/ingest`.

## Fase 3 - SDK Web

### TASK-ST-0301: Agregar tipos y helpers de feature

Estado: `done`

Requisitos: `REQ-ST-014`, `REQ-ST-015`, `REQ-ST-017`, `REQ-ST-028`

Criterios: `AC-ST-007`, `AC-ST-012`

Acciones:

- Definir tipos de payload para `feature_opened` y `feature_action_performed`.
- Agregar metodos al contrato `AnalyticsClient`.
- Implementar helpers delegando a `track`.
- Agregar tests que validen `event_name`, `properties`, `context`, `logicalEventId` e `idempotencyKey`.

Evidencia:

- 2026-06-27: `packages/web-sdk/src/index.ts` agrega tipos y metodos `featureOpened` y `featureActionPerformed` delegando en `track`.
- 2026-06-27: `packages/web-sdk/test/behavior-helpers.test.mjs` valida `event_name`, `properties`, `context`, `logicalEventId` e `idempotencyKey`.
- 2026-06-27: Validacion ejecutada: `npm test` en `packages/web-sdk`.

### TASK-ST-0302: Agregar tipos y helpers de busqueda y frustracion

Estado: `done`

Requisitos: `REQ-ST-006`, `REQ-ST-008`, `REQ-ST-014`, `REQ-ST-015`, `REQ-ST-017`, `REQ-ST-028`

Criterios: `AC-ST-007`, `AC-ST-008`, `AC-ST-012`

Acciones:

- Definir tipos de payload para busqueda y frustracion.
- Implementar helpers como delegacion a `track`.
- Documentar `search_term` como opt-in seguro.
- Agregar tests de payload minimo, contexto opcional y opciones de idempotencia.

Evidencia:

- 2026-06-27: `packages/web-sdk/src/index.ts` agrega helpers `searchPerformed`, `searchResultSelected`, `searchEmptyResult`, `searchAbandoned`, `rageClickDetected`, `deadClickDetected` y `flowAbandoned`.
- 2026-06-27: `packages/web-sdk/README.md` documenta `search_term` como opt-in seguro y contexto opcional.
- 2026-06-27: `packages/web-sdk/test/behavior-helpers.test.mjs` valida payload minimo y contexto opcional de busqueda/frustracion.
- 2026-06-27: Validacion ejecutada: `npm test` en `packages/web-sdk`.

### TASK-ST-0303: Mantener sanitizacion de formularios

Estado: `done`

Requisitos: `REQ-ST-007`, `REQ-ST-016`, `REQ-ST-028`

Criterios: `AC-ST-004`, `AC-ST-008`, `AC-ST-012`

Acciones:

- Revisar helpers existentes de formulario.
- Agregar tests para claves bloqueadas y nombres de campo inseguros.
- Actualizar README del SDK con reglas de formularios y contexto opcional.

Evidencia:

- 2026-06-27: Se conservaron los helpers existentes de formulario y su sanitizacion previa.
- 2026-06-27: `packages/web-sdk/test/behavior-helpers.test.mjs` cubre claves bloqueadas (`value`, `token`) y nombres de campo inseguros (`email`, `password`) sin enviar valores de usuario.
- 2026-06-27: `packages/web-sdk/README.md` mantiene la regla de formularios sin valores reales ingresados por usuarios.
- 2026-06-27: Validacion ejecutada: `npm test` en `packages/web-sdk`.

## Fase 4 - Persistencia Y Normalizacion

### TASK-ST-0401: Validar persistencia cruda de contexto

Estado: `done`

Requisitos: `REQ-ST-018`, `REQ-ST-019`, `REQ-ST-028`

Criterios: `AC-ST-009`, `AC-ST-012`

Acciones:

- Agregar o ajustar tests del repositorio/use case para confirmar persistencia de `context`.
- Verificar que eventos actuales sin `context` persisten `{}`.
- Confirmar que eventos de items actuales siguen normalizandose igual.

Evidencia:

- 2026-06-27: `services/worker/internal/adapters/outbound/postgres/event_repository.go` ya serializa `properties` y `context` como JSONB no nulo en `analytics_events`.
- 2026-06-27: `services/worker/internal/application/usecases/process_events_test.go` valida preservacion de `context` crudo anidado y `context` vacio para eventos sin contexto.
- 2026-06-27: `item_impression` sigue generando `ItemDetails` normalizados.
- 2026-06-27: Validacion ejecutada: `go test ./...` en `services/worker` y `services/ingest`.

### TASK-ST-0402: Decidir normalizacion de `item_selected_for_context`

Estado: `done`

Requisitos: `REQ-ST-009`, `REQ-ST-020`, `REQ-ST-021`

Criterios: `AC-ST-010`

Acciones:

- Evaluar si v1 requiere fila en `analytics_event_items` o solo persistencia cruda.
- Si se normaliza, agregar reglas sin campos de dominio y con tests.
- Si no se normaliza, documentar consulta via `analytics_events.context` y postergar migracion.

Evidencia:

- 2026-06-27: Decision v1: `item_selected_for_context` queda persistido como evento crudo en `analytics_events`, sin fila en `analytics_event_items`.
- 2026-06-27: `services/worker/internal/application/usecases/item_details.go` acota la normalizacion a eventos de items soportados: `item_impression`, `item_viewed`, `item_image_zoomed`, `cart_item_added`, `checkout_started` y `purchase_completed`.
- 2026-06-27: `process_events_test.go` valida que `item_selected_for_context` conserva `properties` y `context` pero no crea `ItemDetails`.
- 2026-06-27: `docs/event-contract.md` mantiene documentado que `item_selected_for_context` es crudo en v1 salvo extension explicita.
- 2026-06-27: Validacion ejecutada: `go test ./...` en `services/worker`.

## Fase 5 - Agregados Opcionales

### TASK-ST-0501: Preparar decision de agregados contextuales

Estado: `done`

Requisitos: `REQ-ST-022`, `REQ-ST-023`, `REQ-ST-024`, `REQ-ST-025`

Criterios: `AC-ST-010`, `AC-ST-013`

Acciones:

- Confirmar si los agregados se implementan en `GoAnalytics` o fuera del servicio.
- Si entran en `GoAnalytics`, disenar migracion, job, algoritmo versionado y endpoint con credencial de lectura.
- Si quedan fuera, documentar contrato de lectura/export y no crear endpoint.

Evidencia:

- 2026-06-27: Decision v1 coordinada: GoAnalytics no expone endpoint online de agregados ni crea tabla/job de top items por contexto.
- 2026-06-27: `docs/next_implementation/_complete/system_tracking/04_contrato_agregados_y_consulta.md` documenta la decision, responsabilidades de GoAnalytics y responsabilidades del integrador.
- 2026-06-27: `docs/next_implementation/_complete/system_tracking/05_cambios_recomendados_goanalytics.md` explicita que no se crean migraciones, jobs, endpoint ni `query_jwt` de agregados en v1.
- 2026-06-27: Validacion documental: no se agregaron migraciones ni endpoints para agregados.

## Fase 6 - Referencia De Contratos Y Validacion Final

### TASK-ST-0601: Documentar contratos para integradores y ejecutar validacion final

Estado: `done`

Requisitos: `REQ-ST-002`, `REQ-ST-010`, `REQ-ST-026`, `REQ-ST-027`, `REQ-ST-028`, `REQ-ST-029`

Criterios: `AC-ST-001`, `AC-ST-005`, `AC-ST-011`, `AC-ST-012`, `AC-ST-014`

Acciones:

- Crear `docs/integration/system-tracking-contracts.md` como referencia para proyectos integradores.
- Detallar endpoint de ingesta, autenticacion, forma batch, campos comunes, `properties`, `context`, eventos soportados, ejemplos de payload, reglas de seguridad, deduplicacion y respuestas esperadas.
- Incluir el contrato de agregados si se implemento; si queda fuera de v1, documentar explicitamente que es fase posterior y como consumir eventos crudos mientras tanto.
- Ejecutar tests Go de `services/ingest` y `services/worker`.
- Ejecutar tests del SDK.
- Revisar documentacion en espanol y sin acoplamiento a integradores.
- Verificar que la referencia de contratos sea autosuficiente para que otro proyecto emita eventos sin leer el SDD interno.
- Verificar que no se agregaron migraciones o endpoints fuera de alcance sin decision explicita.

Evidencia:

- 2026-06-27: Se creo `docs/integration/system-tracking-contracts.md` como referencia autosuficiente para integradores.
- 2026-06-27: La referencia documenta endpoint de ingesta, autenticacion, forma batch, campos comunes, `properties`, `context`, eventos soportados, seguridad, deduplicacion, persistencia y decision de agregados fuera de v1.
- 2026-06-27: Validacion ejecutada: `make test` desde la raiz del repo.
- 2026-06-27: Validacion ejecutada: `npm test` en `packages/web-sdk`.
- 2026-06-27: Revision documental ejecutada: `rg -n -i "\b(dragonfullav|erp|crm|cms|pos)\b" docs/integration/system-tracking-contracts.md docs/event-contract.md docs/event-conventions.md` sin resultados.
- 2026-06-27: No se agregaron migraciones ni endpoints de agregados en GoAnalytics v1.
