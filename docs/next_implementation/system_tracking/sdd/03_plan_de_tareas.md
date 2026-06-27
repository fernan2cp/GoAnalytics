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

Estado: `pending`

Requisitos: `REQ-ST-010`, `REQ-ST-012`, `REQ-ST-013`, `REQ-ST-028`

Criterios: `AC-ST-005`, `AC-ST-006`, `AC-ST-012`

Acciones:

- Agregar tests para claves sensibles en `context` de primer nivel.
- Agregar tests para claves sensibles anidadas en `context`.
- Verificar que campos desconocidos seguros en `context` no rechazan eventos.
- Mantener tests equivalentes para `properties`.

Evidencia:

- pendiente.

### TASK-ST-0202: Definir y probar limites de payload contextual

Estado: `pending`

Requisitos: `REQ-ST-011`, `REQ-ST-013`, `REQ-ST-028`

Criterios: `AC-ST-006`, `AC-ST-012`

Acciones:

- Revisar limites actuales de batch y payload entre SDK, ingesta y worker.
- Definir limite objetivo para tamanio, profundidad o cantidad de claves si falta cobertura backend.
- Implementar validacion en capa de dominio o aplicacion si corresponde.
- Agregar tests con contexto grande aceptado dentro del limite y rechazado fuera del limite.

Evidencia:

- pendiente.

## Fase 3 - SDK Web

### TASK-ST-0301: Agregar tipos y helpers de feature

Estado: `pending`

Requisitos: `REQ-ST-014`, `REQ-ST-015`, `REQ-ST-017`, `REQ-ST-028`

Criterios: `AC-ST-007`, `AC-ST-012`

Acciones:

- Definir tipos de payload para `feature_opened` y `feature_action_performed`.
- Agregar metodos al contrato `AnalyticsClient`.
- Implementar helpers delegando a `track`.
- Agregar tests que validen `event_name`, `properties`, `context`, `logicalEventId` e `idempotencyKey`.

Evidencia:

- pendiente.

### TASK-ST-0302: Agregar tipos y helpers de busqueda y frustracion

Estado: `pending`

Requisitos: `REQ-ST-006`, `REQ-ST-008`, `REQ-ST-014`, `REQ-ST-015`, `REQ-ST-017`, `REQ-ST-028`

Criterios: `AC-ST-007`, `AC-ST-008`, `AC-ST-012`

Acciones:

- Definir tipos de payload para busqueda y frustracion.
- Implementar helpers como delegacion a `track`.
- Documentar `search_term` como opt-in seguro.
- Agregar tests de payload minimo, contexto opcional y opciones de idempotencia.

Evidencia:

- pendiente.

### TASK-ST-0303: Mantener sanitizacion de formularios

Estado: `pending`

Requisitos: `REQ-ST-007`, `REQ-ST-016`, `REQ-ST-028`

Criterios: `AC-ST-004`, `AC-ST-008`, `AC-ST-012`

Acciones:

- Revisar helpers existentes de formulario.
- Agregar tests para claves bloqueadas y nombres de campo inseguros.
- Actualizar README del SDK con reglas de formularios y contexto opcional.

Evidencia:

- pendiente.

## Fase 4 - Persistencia Y Normalizacion

### TASK-ST-0401: Validar persistencia cruda de contexto

Estado: `pending`

Requisitos: `REQ-ST-018`, `REQ-ST-019`, `REQ-ST-028`

Criterios: `AC-ST-009`, `AC-ST-012`

Acciones:

- Agregar o ajustar tests del repositorio/use case para confirmar persistencia de `context`.
- Verificar que eventos actuales sin `context` persisten `{}`.
- Confirmar que eventos de items actuales siguen normalizandose igual.

Evidencia:

- pendiente.

### TASK-ST-0402: Decidir normalizacion de `item_selected_for_context`

Estado: `pending`

Requisitos: `REQ-ST-009`, `REQ-ST-020`, `REQ-ST-021`

Criterios: `AC-ST-010`

Acciones:

- Evaluar si v1 requiere fila en `analytics_event_items` o solo persistencia cruda.
- Si se normaliza, agregar reglas sin campos de dominio y con tests.
- Si no se normaliza, documentar consulta via `analytics_events.context` y postergar migracion.

Evidencia:

- pendiente.

## Fase 5 - Agregados Opcionales

### TASK-ST-0501: Preparar decision de agregados contextuales

Estado: `pending`

Requisitos: `REQ-ST-022`, `REQ-ST-023`, `REQ-ST-024`, `REQ-ST-025`

Criterios: `AC-ST-010`, `AC-ST-013`

Acciones:

- Confirmar si los agregados se implementan en `GoAnalytics` o fuera del servicio.
- Si entran en `GoAnalytics`, disenar migracion, job, algoritmo versionado y endpoint con credencial de lectura.
- Si quedan fuera, documentar contrato de lectura/export y no crear endpoint.

Evidencia:

- pendiente.

## Fase 6 - Referencia De Contratos Y Validacion Final

### TASK-ST-0601: Documentar contratos para integradores y ejecutar validacion final

Estado: `pending`

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

- pendiente.
