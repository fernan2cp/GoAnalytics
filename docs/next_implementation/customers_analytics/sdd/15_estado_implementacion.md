# Estado De Implementación

## 1. Estado Del Paquete

Al 2026-11-13 se completaron las 36 tareas (28 originales, dos de configuración y seis de corrección/recertificación). La sección 8 contiene la evidencia vigente y prevalece sobre las mediciones históricas de agosto. El MVP queda aceptado para rollout progresivo; las migraciones y activación de tenants de negocio son un paso operativo posterior, no ejecutado por esta certificación.

Estados permitidos:

- `pending`: no iniciada o sin evidencia suficiente.
- `in_progress`: implementación activa; todavía no cumple todos sus criterios.
- `blocked`: impedimento concreto registrado.
- `done`: implementación y criterios aceptados con evidencia.

## 2. Tablero

| Fase | Tarea | Resumen | Responsable | Estado | Evidencia |
|---|---|---|---|---|---|
| 0 | `TASK-CA-0001` | Recuperar pendientes Redis | GoAnalytics | `done` | Worker e integración Redis aprobados; `AC-CA-002` y `AC-CA-003` cubiertos |
| 0 | `TASK-CA-0002` | Aislar persistencia e idempotencia | GoAnalytics | `done` | `AC-CA-004` cubierto; regresión de `AC-CA-001` preservada |
| 0 | `TASK-CA-0003` | Flags, configuración y migración `005` | GoAnalytics | `done` | Migración `005` up/down desde vacío y `004`; matriz opt-in aprobada |
| 0 | `TASK-CA-0004` | Extender resolver de sites | Ambos proyectos | `done` | Contrato Go/Dragon, caché legacy, server sin origin y timezone IANA aprobados; Dragon `4bcf56d13` |
| 0 | `TASK-CA-0005` | Preparar operación y rendimiento | GoAnalytics | `done` | Métricas/health, dashboard, alertas, runbook, imágenes y perfil de carga preparados |
| 1 | `TASK-CA-0101` | Modelo de sujetos y migración `006` | GoAnalytics | `done` | Migración `006` up/down, convergencia concurrente, restricciones tenant-first e invariantes de dominio aprobadas |
| 1 | `TASK-CA-0102` | Endpoint interno de eventos | GoAnalytics | `done` | Bearer dedicado, esquema server estricto, publicación al stream compartido y regresión pública aprobados |
| 1 | `TASK-CA-0103` | Propagar origen, timezone y trazabilidad | GoAnalytics | `done` | Recorrido web/server, resolución tenant/site, snapshot timezone/source y persistencia trazable aprobados |
| 1 | `TASK-CA-0104` | Normalización 1:N tenant-first | GoAnalytics | `done` | Normalización opt-in, atomicidad, convergencia concurrente, fallos semánticos e idempotencia entre sites aprobados |
| 1 | `TASK-CA-0105` | Lifecycle monotónico y merge | GoAnalytics | `done` | Precedencia compuesta, no-op auditable, deactivate/reactivate, merge y elegibilidad aprobados |
| 1 | `TASK-CA-0106` | Emisión desde proyecto principal | Proyecto principal | `done` | Emisor post-commit opt-in, retry directo, idempotencia, lifecycle y degradación aprobados; Dragon `2747c0fdf` |
| 2 | `TASK-CA-0201` | Snapshot revisionado | Proyecto principal | `done` | Corte inmutable por 2 h, cursor firmado, límite 500, auth y expiración aprobados; Dragon `923a7d221` |
| 2 | `TASK-CA-0202` | Cliente de snapshot y bootstrap | GoAnalytics | `done` | `subject-jobs`, cliente con retry, checkpoint transaccional, reanudación y revisión expirada aprobados |
| 2 | `TASK-CA-0203` | Convergencia de identidades | GoAnalytics | `done` | Unicidad tenant-first, precedencia monotónica y carreras lazy/backfill/lifecycle aprobadas en PostgreSQL real |
| 2 | `TASK-CA-0204` | Certificar backfill inicial | Ambos proyectos | `done` | 100.000 sujetos en 1m11,514s; reanudación al 50 %, completitud y cero duplicados aprobados |
| 3 | `TASK-CA-0301` | Migración `007` y repositorios | GoAnalytics | `done` | Migración 007 up/down y repositorio transaccional de runs, checkpoint y agregado aprobados |
| 3 | `TASK-CA-0302` | Proyector global incremental | GoAnalytics | `done` | Advisory lock, watermark compuesto, recomputación atómica, tardíos y restart aprobados |
| 3 | `TASK-CA-0303` | Programar `subject-jobs` | GoAnalytics | `done` | Modos bootstrap/project-global/serve, ejecución al inicio, scheduler diario y operación HTTP aprobados |
| 3 | `TASK-CA-0304` | Query API y ranking global | GoAnalytics | `done` | Proceso separado, Bearer, validación, stale/unavailable/vacío, orden y aislamiento aprobados |
| 3 | `TASK-CA-0305` | Consumir sugerencias globales | Proyecto principal | `done` | Cliente opt-in, hidratación local, elegibilidad, orden y degradación aprobados; Dragon `d0216f882` |
| 4 | `TASK-CA-0401` | Publicar registro de contextos | Proyecto principal | `done` | Siete familias, revisión estable, inmutabilidad, ciclos y endpoint autenticado aprobados; Dragon `1c576f810` |
| 4 | `TASK-CA-0402` | Migración `008`, registro y caché | GoAnalytics | `done` | `AC-CA-017` y `AC-CA-018` cubiertos; migración reversible, registro revisionado y lock Redis validados |
| 4 | `TASK-CA-0403` | Resolver contextos al normalizar | GoAnalytics | `done` | Resolución exacta, revisión histórica, inactivos y desconocidos validados |
| 4 | `TASK-CA-0404` | Resolver históricos iniciales | GoAnalytics | `done` | Job paginado, checkpoint por revisión y reejecución no-op validados |
| 4 | `TASK-CA-0405` | Proyección contextual | GoAnalytics | `done` | Lock, watermark, recomposición atómica y scheduler conjunto validados |
| 4 | `TASK-CA-0406` | Ranking contextual y fallback | GoAnalytics | `done` | Ranking exacto tenant-first y fallback único a global validados |
| 4 | `TASK-CA-0407` | Integración contextual principal | Proyecto principal | `done` | DragonFullAV `9554c5e6b`; emisión contextual, hidratación, flags y fallback validados |
| 4 | `TASK-CA-0408` | Certificar MVP | Ambos proyectos | `done` | Regresión, integraciones, E2E, volumen, imágenes y rollback aprobados |

## 3. Gates De Fase

| Gate | Condición Para Aprobar | Estado Inicial |
|---|---|---|
| Fase 0 → 1 | Pendientes recuperables, conflictos aislados, migración `005`, resolver aditivo y flags seguros | `done` |
| Fase 1 → 2 | Endpoint interno, identidad, normalización, lifecycle y emisor post-commit aceptados | `done` |
| Fase 2 → 3 | Snapshot estable, bootstrap reanudable y backfill certificado | `done` |

## 3.1. Certificación Del Gate De Fase 2

| Dato | Resultado |
|---|---|
| Fecha y entorno | 2026-08-11, PostgreSQL local `go-analytics-postgres` |
| Comando | `SUBJECT_VOLUME_TEST=1 go test ./internal/adapters/outbound/postgres -run TestSubjectBootstrapVolume100k -v -count=1` |
| Volumen | 20 tenants, 100.000 clientes, páginas de 500 |
| Distribución | 90.000 active, 10.000 inactive |
| Reanudación | checkpoint exacto en 50.000 y continuación sin duplicados |
| Duración | 1m11,514s; 1.398,33 sujetos/s |
| Regresión | ingest y worker completos; 16/16 pruebas SDK |
| Decisión | gate aprobado; Fase 3 habilitada, capacidades productivas todavía opt-in |
| Fase 3 → 4 | Proyección global, query API e integración global dentro de SLO | `done` |
| Cierre MVP | Registro, resolución inicial, ranking contextual, rollout y regresión completa aceptados | `done` |

## 3.2. Certificación Del Cierre MVP

| Dato | Resultado |
|---|---|
| Fecha y entorno | 2026-08-11; Go local, PostgreSQL 16 y Redis 7 en Docker Desktop |
| Regresión | ingest y worker completos con vet; SDK 16/16; DragonFullAV analytics 12/12 |
| Integración durable | PostgreSQL 12/12 y Redis 13/13 sobre servicios reales |
| E2E técnico | asociación interna → normalización → global/contexto → sugerencia exacta |
| Perfil máximo | 20 tenants, 100.000 sujetos, 500.000 eventos |
| Proyección | 9,310 s global + contextual; objetivo ≤30 min |
| Query | p95 17,263 ms para `limit=10`; objetivo ≤300 ms |
| Backfill | 1m11,514 s; objetivo ≤60 min |
| Artefactos | imágenes `subject-jobs` y `query-api` construidas correctamente |
| Rollback | flags apagados y detención de binarios; sin down destructivo en producción |
| Decisión | MVP aprobado para rollout progresivo por tenant |

## 4. Decisiones Cerradas Para El MVP

- Lifecycle se ordena por `source_updated_at + source_mutation_id`.
- La fuente entrega con retry directo de tres intentos, sin outbox.
- El snapshot incluye active, inactive y merged, con revisión estable.
- Jobs y query API son binarios separados dentro del módulo worker.
- Se usan siete familias iniciales y fallback exacto → global.
- La frescura objetivo es diaria; stale comienza a las 48 horas y se sirve por defecto.
- La conciliación general y las evoluciones avanzadas quedan fuera del MVP.

## 5. Cómo Actualizar El Tablero

Para cambiar una tarea a `done` se debe:

1. enlazar commit o PR de cada repositorio afectado;
2. enlazar evidencia de todos sus `AC-CA-*`;
3. registrar migraciones y flags aplicados;
4. indicar entorno y fecha de validación;
5. actualizar el gate de fase si todas sus tareas están aceptadas.

No se debe inferir avance por la sola existencia de código: sin evidencia reproducible, el estado permanece `pending`.

## 6. Tareas Adicionales De Configuración

| Fase | Tarea | Resumen | Responsable | Estado | Evidencia |
|---|---|---|---|---|---|
| Configuración | `TASK-CA-CFG-0001` | Backup previo del compose de GoAnalytics | GoAnalytics | `done` | Backup byte a byte fuera del repositorio; 3.346 bytes y SHA-256 `0AAAC09AA127203C787F4901BD807C92CFE50FAEA38D1C4F8F930450A273BA8F` verificados antes de editar el compose |
| Configuración | `TASK-CA-CFG-0002` | Instructivo resumido para producción | GoAnalytics | `done` | `docs/next_implementation/customers_analytics/instructivo_configuracion_mvp_produccion.md` versionado; variables, procesos, smoke tests, rollback y checklist documentados sin secretos |

## 7. Bloqueo Técnico Histórico — Resuelto

| Hallazgo | Estado | Evidencia | Acción requerida |
|---|---|---|---|
| El perfil server enviaba `token_version=0` al resolver | `done` | `TASK-CA-0501` normaliza la resolución a versión 1; `TASK-CA-0506` certifica asociación y lifecycle cruzados | Desplegar los commits corregidos y seguir el instructivo antes de activar tenants |

## 8. Recertificación Vigente — 2026-11-13

| Tarea | Estado | Evidencia |
|---|---|---|
| `TASK-CA-0501` | `done` | `2dcef46`: contrato server corregido sin ampliar el endpoint interno |
| `TASK-CA-0502` | `done` | `683b0bf`: refresh real, memoria con TTL y caché con IDs; PostgreSQL verifica inmutabilidad y revisión creciente |
| `TASK-CA-0503` | `done` | `b25a8c0`: lock de importación, reanudación, expiración explícita y `--force-new-snapshot` |
| `TASK-CA-0504` | `done` | GoAnalytics `ea4518d`; DragonFullAV `d39389878` y `898f613f7`: versión lifecycle persistida y captura SQL opt-in |
| `TASK-CA-0505` | `done` | `e12e97a`: recuperación de proyección contextual y frescura de la fuente servida |
| `TASK-CA-0506` | `done` | E2E reproducible, regresión, carga HTTP distribuida, métricas, artefactos y rollback; DragonFullAV `d3b998103`, `aa00fffa5` |

| Dato | Resultado actual |
|---|---|
| Entorno | Windows amd64, Go 1.26.2; Docker Desktop; PostgreSQL 18.6, Redis 7.4.7 locales aislados |
| Regresión | Ingest y worker con vet; worker con PostgreSQL/Redis; SDK 16/16; DragonFullAV analytics 26/26 |
| Recorrido cruzado | Emisor y bordes reales; metadata de sites de fixture; SQL fuente y binarios Go reales, sin detalles normalizados precargados |
| Migraciones | GoAnalytics 001→008 desde vacío y sobre 004 con legacy; down hasta vacío. Fuente custanalytics01 up/down en schema aislado |
| Perfil | 20 tenants, 100.000 clientes, 500.000 asociaciones repartidas; totales de agregados comprobados |
| Backfill | 59,732 s; objetivo ≤60 min |
| Proyección global + contextual | 4m20,496s; objetivo ≤30 min |
| API HTTP p95 | 23,785 ms, limit 10, 200 consultas; incluye Bearer, SQL y serialización; objetivo ≤300 ms |
| Rollback | Todos los flags de sujetos apagados, endpoint interno 404 y evento web persistido; tablas conservadas |
| Operación | Instructivo versionado; captura lifecycle SQL apagada por defecto; activación explícita por tenant y flags de aplicación |
| Limitación operativa | Snapshot en memoria: una instancia/afinidad durante paginación; sin outbox ni conciliación general, según alcance MVP |

Comandos reproducibles en `11_plan_pruebas.md`. El E2E elimina únicamente su base efímera y limpia su Redis dedicado. Los números de carga describen este entorno local y no sustituyen la observación del rollout en producción.
