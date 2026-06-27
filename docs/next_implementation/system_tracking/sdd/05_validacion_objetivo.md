# Validacion Objetivo

## Comandos

Ejecutar desde la raiz del repo:

```powershell
go test ./services/ingest/...
```

```powershell
go test ./services/worker/...
```

```powershell
npm test --prefix packages/web-sdk
```

## Mapa De Criterios A Evidencia

- `AC-ST-001`: tests de ingesta y casos minimos de eventos sin `context`, con `context` vacio y con alias.
- `AC-ST-002`: inspeccion de `docs/event-contract.md` y `docs/event-conventions.md`.
- `AC-ST-003`: inspeccion de documentacion publica y tests de eventos genericos.
- `AC-ST-004`: tests del SDK sobre sanitizacion de formularios e inspeccion de ejemplos.
- `AC-ST-005`: tests del worker con claves sensibles en `properties` y `context`.
- `AC-ST-006`: tests backend con campos desconocidos seguros y payloads dentro/fuera de limites.
- `AC-ST-007`: tests del SDK que capturan el batch generado por cada helper.
- `AC-ST-008`: tests del SDK con payloads opcionales faltantes y formularios sanitizados.
- `AC-ST-009`: tests de worker/repositorio o integracion PostgreSQL sobre persistencia JSONB.
- `AC-ST-010`: inspeccion de la decision documentada o de la implementacion de normalizacion/agregados.
- `AC-ST-011`: inspeccion de documentacion para ejemplos genericos y sin acoplamiento.
- `AC-ST-012`: ejecucion completa de comandos Go y SDK.
- `AC-ST-013`: tests o inspeccion operacional de job/endpoint de agregados si esa fase se implementa.
- `AC-ST-014`: inspeccion de `docs/integration/system-tracking-contracts.md` como referencia autosuficiente para integradores.

## Inspecciones De Documentacion

- Verificar que `docs/event-contract.md` describe `context` generico opcional y mantiene compatibilidad con eventos sin `context`.
- Verificar que `docs/event-conventions.md` contiene reglas de seguridad para `properties` y `context`.
- Verificar que `packages/web-sdk/README.md` documenta helpers nuevos como delegacion a `track`.
- Verificar que `docs/integration/system-tracking-contracts.md` existe y contiene endpoint de ingesta, autenticacion, forma batch, campos comunes, contratos por evento, reglas de seguridad, deduplicacion y respuestas esperadas.
- Verificar que `docs/integration/system-tracking-contracts.md` indica el estado de agregados contextuales: contrato implementado o fase posterior.
- Verificar que los ejemplos no contienen datos personales, secretos, documentos, tarjetas ni texto libre sensible.
- Verificar que los ejemplos no dependen de dominios ERP, CRM, POS, CMS ni integradores especificos.

## Inspecciones De Codigo

- Revisar `services/ingest/internal/adapters/inbound/http/request_mapper.go` para confirmar alias y `context` opcional.
- Revisar `services/ingest/internal/application/usecases/ingest_events.go` para confirmar que no se agregan validaciones de catalogo cerrado.
- Revisar `services/worker/internal/application/usecases/payload_safety.go` para confirmar bloqueo en `properties` y `context`.
- Revisar `services/worker/internal/adapters/outbound/postgres/event_repository.go` para confirmar persistencia JSONB de `properties` y `context`.
- Revisar `packages/web-sdk/src/index.ts` para confirmar que helpers nuevos delegan en `track`.

## Casos Minimos A Cubrir

- Evento actual sin `context` aceptado.
- Evento actual con `context: {}` aceptado.
- Evento con `context.app_area`, `context.feature`, `context.surface`, `context.entry_point`, `context.component_id`, `context.flow_id`, `context.entity_type`, `context.entity_id` y `context.runtime` aceptado.
- Evento con campo desconocido seguro en `context` aceptado.
- Evento con `context.password` rechazado.
- Evento con `context.runtime.token` rechazado.
- Evento con `properties.password` rechazado.
- `feature_opened` aceptado con payload minimo.
- `feature_action_performed` aceptado con payload minimo.
- `search_performed` aceptado con `query_length` sin `search_term`.
- `search_result_selected` aceptado con `search_id`, `result_type`, `result_id` y `position`.
- `search_empty_result` aceptado.
- `search_abandoned` aceptado.
- `form_validation_attempt`, `form_completed` y `form_abandoned` sanitizan campos de usuario.
- `rage_click_detected`, `dead_click_detected` y `flow_abandoned` aceptados con payload minimo.
- `item_selected_for_context` queda persistido en `analytics_events`; si se decide normalizarlo, tambien se valida fila esperada en `analytics_event_items`.

## Validacion De Agregados Si Se Implementan

- El job no corre dentro del handler de ingesta.
- La salida separa `tenant_id` y `site_id`.
- El algoritmo escribe o devuelve `algorithm_version`.
- La consulta respeta `limit`, ventana temporal y filtros parciales.
- La respuesta no incluye datos personales.
- El endpoint rechaza tracking JWT publico y exige credencial de lectura.

## Bloqueos Conocidos

- Si se decide implementar agregados contextuales dentro de `GoAnalytics`, falta una decision de alcance sobre tabla/vista, job, endpoint y credencial. Hasta cerrar esa decision, el SDD considera agregados como fase opcional.
- Si se decide copiar contexto a `analytics_event_items`, falta definir si se agrega `surface_context` o si se reutiliza `surface` con semantica unica documentada.
