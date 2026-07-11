# Auditoria Y Deuda Tecnica: System Tracking

## Alcance Y Fecha

Auditoria realizada el 2026-07-11 sobre toda la documentacion archivada en `_complete/system_tracking/`, incluyendo `sdd/`, cuya ubicacion inicial fue la carpeta `system_tracking` de `next_implementation`, y sobre:

- mapeo HTTP de ingesta;
- casos de uso de ingesta y worker;
- validacion y seguridad de payload;
- persistencia PostgreSQL;
- migraciones `001` a `004`;
- SDK web y sus pruebas;
- documentacion publica de contratos.

La propuesta `07_propuesta_evolucion_futura_agregados.md` fue auditada como propuesta de mejora futura y no como deuda tecnica.

## Resumen Y Puerta De Archivo

| Estado | Cantidad |
|---|---:|
| `RESUELTO` | 6 |
| `PENDIENTE` | 0 |
| `MITIGADO` | 1 |
| `NO_VERIFICADO` | 0 |
| `DESCARTADO` | 2 |

La puerta de archivo se supera. La falta de una prueba automatizada de extremo a extremo contra PostgreSQL se mantiene como riesgo mitigado, no como comportamiento no verificado, porque el contrato, el caso de uso, el repositorio SQL y el esquema real fueron contrastados.

## Pendientes Y Riesgos Mitigados

### ST-007 - Cobertura De Persistencia PostgreSQL

- **Estado:** `MITIGADO`.
- **Severidad:** media.
- **Origen:** `sdd/04_criterios_de_aceptacion.md`, criterio `AC-ST-009`.
- **Evidencia observada:** `analytics_events.properties` y `analytics_events.context` se escriben en `EventRepository.SaveBatch`; el worker conserva mapas normalizados; las migraciones fueron aplicadas, inspeccionadas, revertidas y reaplicadas en PostgreSQL real.
- **Riesgo residual:** no existe una prueba automatizada que ejecute el flujo completo de worker contra PostgreSQL y lea posteriormente `analytics_events.context`.
- **Trabajo restante:** incorporar una prueba de integracion cuando exista un harness estable para el repositorio PostgreSQL.
- **Validacion futura:** ingerir contexto seguro, persistirlo mediante el adaptador real y verificar lectura exacta, aislamiento y rollback.

## Resueltos

- **ST-001:** aliases `event_type`, `page_url` y `metadata`; implementados en `request_mapper.go` y cubiertos por pruebas.
- **ST-002:** contexto opcional, campos desconocidos seguros, limites de 64 KiB y profundidad 16; implementados en mapper, normalizacion y `payload_safety.go`.
- **ST-003:** bloqueo recursivo de claves sensibles en `properties` y `context`; implementado en worker y cubierto por pruebas.
- **ST-004:** persistencia de `properties` y `context` como JSONB no nulo; implementada en `event_repository.go` y migraciones.
- **ST-005:** helpers genericos de feature, busqueda, frustracion, flujo y formularios; implementados como azucar sobre `track` en el SDK y cubiertos por 16 pruebas.
- **ST-006:** `item_selected_for_context` se conserva como evento crudo sin normalizacion de items en v1; implementado y cubierto por prueba dedicada.

## Decisiones Fuera De Alcance

- **ST-008:** `analytics_context_item_aggregates`, endpoint `GET /v1/aggregates/items/top`, scoring online y `query_jwt`; `DESCARTADO` por la decision coordinada de v1 en el contrato de agregados.
- **ST-009:** propuesta de evolucion futura hacia proyecciones batch y consulta generica; `DESCARTADO` como deuda y conservado como propuesta independiente en `07_propuesta_evolucion_futura_agregados.md`.

## Matriz De Trazabilidad

| ID | Fuente | Obligacion auditada | Estado | Evidencia |
|---|---|---|---|---|
| ST-001 | Contrato HTTP | Aceptar aliases y extras compatibles | `RESUELTO` | `request_mapper.go`, pruebas HTTP |
| ST-002 | SDD 01-02 | Aceptar contexto seguro y opcional | `RESUELTO` | mapper, `NormalizeMap`, pruebas |
| ST-003 | SDD 02, seguridad | Bloquear claves sensibles anidadas | `RESUELTO` | `payload_safety.go`, pruebas worker |
| ST-004 | SDD 02, persistencia | Guardar `properties` y `context` completos | `RESUELTO` | repositorio, migracion `001`, inspeccion DB |
| ST-005 | SDD 02, SDK | Exponer helpers sin transporte paralelo | `RESUELTO` | `packages/web-sdk/src/index.ts`, 16 pruebas |
| ST-006 | SDD 03-05 | Mantener seleccion contextual como evento crudo | `RESUELTO` | `item_details.go`, prueba worker |
| ST-007 | SDD 04, `AC-ST-009` | Validar persistencia real y lectura de contexto | `MITIGADO` | SQL y esquema reales; falta harness automatizado |
| ST-008 | Contrato de agregados | Mantener agregados online fuera de v1 | `DESCARTADO` | decision v1 documentada |
| ST-009 | Propuesta 07 | Evaluar evolucion futura de agregados | `DESCARTADO` | propuesta archivada separadamente; no es deuda |

## Evidencia De Validacion

- `go test ./services/ingest/...`: correcto.
- `go test ./services/worker/...`: correcto.
- `npm test --prefix packages/web-sdk`: correcto, 16 pruebas.
- PostgreSQL saludable mediante `docker compose`.
- Migraciones `001` a `004`: aplicadas correctamente.
- Tablas `analytics_events`, `analytics_event_items` y `analytics_event_orders`: presentes.
- Claves foraneas e indices esperados: presentes.
- Reversion y reaplicacion de migracion `004`: correctas.
- Escaneo de referencias antes del movimiento: completado.

## Decisión De Archivado

La documentacion de system tracking se archiva en `_complete/system_tracking/`. La propuesta `07_propuesta_evolucion_futura_agregados.md` permanece como documento independiente dentro de esa carpeta. Este archivo de deuda tecnica permanece en la ubicacion original.
