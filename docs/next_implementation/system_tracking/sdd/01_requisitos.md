# Requisitos

## Contrato De Contexto

- `REQ-ST-001`: El sistema debe aceptar `context` como objeto JSON opcional en eventos existentes y nuevos, sin exigir catalogos cerrados de valores.
- `REQ-ST-002`: El sistema debe preservar compatibilidad con eventos sin `context`, con `context` vacio y con alias publicos existentes.
- `REQ-ST-003`: El contrato debe documentar campos recomendados de contexto: `app_area`, `feature`, `surface`, `entry_point`, `component_id`, `flow_id`, `entity_type`, `entity_id` y `runtime`.
- `REQ-ST-004`: El sistema debe mantener separada la semantica de `properties` como hecho observado y `context` como entorno funcional o tecnico.

## Eventos De Comportamiento

- `REQ-ST-005`: El contrato debe cubrir eventos genericos de uso: `feature_opened` y `feature_action_performed`.
- `REQ-ST-006`: El contrato debe cubrir eventos genericos de busqueda: `search_performed`, `search_result_selected`, `search_empty_result` y `search_abandoned`.
- `REQ-ST-007`: El contrato debe mantener eventos de formularios existentes y reforzar que no deben incluir valores ingresados por usuarios.
- `REQ-ST-008`: El contrato debe cubrir eventos de frustracion: `rage_click_detected`, `dead_click_detected` y `flow_abandoned`.
- `REQ-ST-009`: El contrato debe permitir `item_selected_for_context` como evento generico de seleccion sin reemplazar los eventos de items existentes.

## Seguridad Y Validacion

- `REQ-ST-010`: La validacion debe bloquear claves sensibles en `properties` y `context`, incluyendo objetos anidados.
- `REQ-ST-011`: La implementacion debe definir limites verificables para payloads con contexto grande.
- `REQ-ST-012`: El contrato debe evitar datos personales, secretos, tarjetas, documentos y texto libre sensible en ejemplos y reglas.
- `REQ-ST-013`: Los eventos con campos de contexto desconocidos pero JSON seguro no deben rechazarse por desconocimiento del campo.

## SDK Web

- `REQ-ST-014`: El SDK debe conservar `track(eventName, properties, options)` como mecanismo base y permitir `options.context`.
- `REQ-ST-015`: Los helpers nuevos de feature y busqueda deben delegar en `track` con `event_name`, `properties` y `context` correctos.
- `REQ-ST-016`: Los helpers de formularios deben conservar sanitizacion local de campos sensibles.
- `REQ-ST-017`: Los helpers deben permitir `logicalEventId` e `idempotencyKey` cuando el integrador tenga identidad funcional estable.

## Persistencia Y Normalizacion

- `REQ-ST-018`: El worker debe persistir el evento crudo completo en `analytics_events.properties` y `analytics_events.context`.
- `REQ-ST-019`: La normalizacion de items debe seguir funcionando para eventos de items actuales sin cambiar semantica.
- `REQ-ST-020`: Si `item_selected_for_context` se normaliza, debe hacerlo sin exigir campos de dominio y con campos contextuales nullable o derivados desde `analytics_events.context`.
- `REQ-ST-021`: Los indices nuevos sobre contexto solo deben agregarse con evidencia de consultas frecuentes o como parte de una fase explicitamente aprobada.

## Agregados Opcionales

- `REQ-ST-022`: Los agregados contextuales, si se implementan, deben ejecutarse fuera del path critico de ingesta.
- `REQ-ST-023`: Los agregados deben separar tenant y site y versionar el algoritmo con `algorithm_version`.
- `REQ-ST-024`: Cualquier endpoint de consulta de agregados debe usar credencial server-to-server o permiso de lectura equivalente.
- `REQ-ST-025`: La respuesta de agregados no debe exponer datos personales; solo IDs tecnicos y metricas agregadas.

## Documentacion Y Compatibilidad

- `REQ-ST-026`: `docs/event-contract.md`, `docs/event-conventions.md` y `packages/web-sdk/README.md` deben describir el contrato generico en espanol.
- `REQ-ST-027`: La documentacion base no debe depender de dominios ERP, CRM, POS, CMS ni de un integrador especifico.
- `REQ-ST-028`: La suite existente de ingesta, worker y SDK debe seguir pasando sin cambios incompatibles.
- `REQ-ST-029`: La implementacion debe entregar una referencia de contratos para proyectos integradores con payloads, campos, reglas de seguridad, deduplicacion, respuestas esperadas y ejemplos de uso.
