# Auditoria Y Deuda Tecnica: Item Event Capture

## Alcance Y Fecha

Auditoria realizada el 2026-07-11 sobre la unidad archivada en `_complete/item_event_capture/`, cuya ubicacion inicial fue la carpeta raiz de `next_implementation`:

- `docs/next_implementation/_complete/item_event_capture/item_event_capture_design.md`.
- `docs/next_implementation/_complete/item_event_capture/item_event_capture_sdd/`.
- `services/worker/internal/application/usecases/item_details.go`.
- `services/worker/internal/application/usecases/deduplication.go`.
- `services/worker/internal/adapters/outbound/postgres/event_repository.go`.
- `migrations/postgres/004_create_item_event_details.up.sql`.
- pruebas unitarias del worker y documentacion publica relacionada.

El objetivo fue contrastar el diseño de captura, normalizacion, deduplicacion y persistencia de eventos de items con el codigo y el esquema real.

## Resumen Y Puerta De Archivo

| Estado | Cantidad |
|---|---:|
| `RESUELTO` | 7 |
| `PENDIENTE` | 0 |
| `MITIGADO` | 1 |
| `NO_VERIFICADO` | 0 |
| `DESCARTADO` | 2 |

La puerta de archivo se supera. El unico riesgo residual es la ausencia de una prueba automatizada que ejecute `SaveBatch` contra PostgreSQL real; queda mitigado por la cobertura del caso de uso, la inspeccion del repositorio SQL y la validacion real de las migraciones.

## Pendientes Y Riesgos Mitigados

### ITEM-008 - Cobertura PostgreSQL De Persistencia

- **Estado:** `MITIGADO`.
- **Severidad:** media.
- **Origen:** `item_event_capture_sdd/06_plan_pruebas.md`, seccion `Pruebas De Persistencia`.
- **Evidencia observada:** el caso de uso tiene cobertura con repositorio falso en `services/worker/internal/application/usecases/item_details_test.go` y `process_events_test.go`; el adaptador implementa la persistencia transaccional en `event_repository.go`; las migraciones fueron aplicadas y revertidas en PostgreSQL real.
- **Riesgo residual:** no existe una prueba automatizada que inserte un evento con uno o varios items mediante el adaptador PostgreSQL y verifique el rollback de la operacion completa.
- **Trabajo restante:** agregar una prueba de integracion PostgreSQL cuando el repositorio incorpore un harness de integracion estable.
- **Validacion futura:** insertar evento simple, evento multi-item y evento con orden; verificar FK, cantidad de filas, `analytics_events.id` y rollback ante error de detalle.

No se clasifica como `NO_VERIFICADO` porque la estructura SQL real, las migraciones y los caminos de persistencia fueron verificados; se conserva el riesgo como mitigado por falta de automatizacion de integracion.

## Resueltos

- **ITEM-001:** extraccion de item unico, lista `items` y payload compatible desde `properties`; implementado en `extractItemPayloads` y cubierto por `item_details_test.go`.
- **ITEM-002:** normalizacion de detalles para los seis eventos soportados; implementada en `normalizesItemDetails` y `itemDetailFromPayload`.
- **ITEM-003:** soporte 1:N para `analytics_event_items`, cabecera opcional para checkout/orden e indices; implementado en la migracion `004`.
- **ITEM-004:** persistencia del evento base y detalles normalizados con `analytics_event_id` como FK interna; implementada en `EventRepository.SaveBatch`.
- **ITEM-005:** deduplicacion especifica para impresiones y lineas de compra; implementada en `itemSpecificDedupKeys` y cubierta por pruebas del worker.
- **ITEM-006:** marcado de eventos incompletos para scoring mediante `missing_fields` e `incomplete_for_scoring`; implementado en `markItemDetailCompleteness` y cubierto por pruebas.
- **ITEM-007:** compatibilidad con eventos sin items y eventos multi-item sin duplicar el evento base; cubierta por extraccion, caso de uso y persistencia por lote.

## Decisiones Fuera De Alcance

- **ITEM-009:** calculo de scores, rankings, reglas de catalogo, stock, diversidad y materializacion tenant; `DESCARTADO` del alcance del microservicio por el diseño original y el SDD.
- **ITEM-010:** particionado, agregados diarios, filtrado avanzado de bots/staff, atribucion avanzada, A/B y normalizacion avanzada de `search_term`; `DESCARTADO` de esta implementacion y reservado para una evaluacion futura.

## Matriz De Trazabilidad

| ID | Fuente | Obligacion auditada | Estado | Evidencia |
|---|---|---|---|---|
| ITEM-001 | Diseño, datos de item | Extraer item unico, lista y formas compatibles | `RESUELTO` | `extractItemPayloads`, pruebas unitarias |
| ITEM-002 | SDD 02 y 03 | Normalizar detalles de los eventos definidos | `RESUELTO` | `normalizesItemDetails`, `itemDetailFromPayload` |
| ITEM-003 | Diseño, tabla `analytics_event_items` | Crear tabla 1:N e indices | `RESUELTO` | migracion `004`, inspeccion PostgreSQL |
| ITEM-004 | SDD 04, persistencia | Guardar evento base, items y orden de forma logica | `RESUELTO` | `EventRepository.SaveBatch`, esquema y pruebas |
| ITEM-005 | Diseño, deduplicacion | Deduplicar impresiones y compras | `RESUELTO` | `itemSpecificDedupKeys`, pruebas del worker |
| ITEM-006 | SDD 02 y 04 | Marcar faltantes sin bloquear auditoria | `RESUELTO` | `markItemDetailCompleteness`, pruebas |
| ITEM-007 | SDD 04, criterios | Mantener eventos sin items y multi-item | `RESUELTO` | extraccion, caso de uso y repositorio |
| ITEM-008 | SDD 06, persistencia | Cubrir integracion real con PostgreSQL | `MITIGADO` | migraciones reales; falta harness automatizado |
| ITEM-009 | Diseño, no responsabilidades | No calcular scoring ni reglas de negocio | `DESCARTADO` | alcance explicito del diseño |
| ITEM-010 | Diseño, mejoras futuras | Evoluciones de rendimiento y atribucion | `DESCARTADO` | seccion de mejoras no bloqueantes |

## Evidencia De Validacion

- `go test ./services/worker/...`: correcto.
- `go test ./services/ingest/...`: correcto como validacion de compatibilidad del contrato general.
- `npm test --prefix packages/web-sdk`: correcto, 16 pruebas.
- `docker compose up -d postgres_analytics`: PostgreSQL saludable.
- Migraciones `001` a `004`: aplicadas correctamente.
- Inspeccion PostgreSQL: tablas, claves foraneas e indices presentes.
- `migrate down 1`: reversion de `004` correcta.
- `migrate up`: reaplicacion de `004` correcta.

## Decisión De Archivado

La documentacion se archiva en `_complete/item_event_capture/`. Este archivo permanece fuera de `_complete` y conserva el unico riesgo residual identificado, sin bloquear el uso de la implementacion actual.
