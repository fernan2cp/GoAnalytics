# Estado De Implementación

## 1. Estado Inicial

Este tablero representa el estado al crear el SDD. Ninguna tarea del MVP fue implementada como parte de este trabajo documental; por eso todas comienzan en `pending` y toda evidencia de ejecución está pendiente.

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
| 0 | `TASK-CA-0004` | Extender resolver de sites | Ambos proyectos | `pending` | Pendiente |
| 0 | `TASK-CA-0005` | Preparar operación y rendimiento | GoAnalytics | `pending` | Pendiente |
| 1 | `TASK-CA-0101` | Modelo de sujetos y migración `006` | GoAnalytics | `pending` | Pendiente |
| 1 | `TASK-CA-0102` | Endpoint interno de eventos | GoAnalytics | `pending` | Pendiente |
| 1 | `TASK-CA-0103` | Propagar origen, timezone y trazabilidad | GoAnalytics | `pending` | Pendiente |
| 1 | `TASK-CA-0104` | Normalización 1:N tenant-first | GoAnalytics | `pending` | Pendiente |
| 1 | `TASK-CA-0105` | Lifecycle monotónico y merge | GoAnalytics | `pending` | Pendiente |
| 1 | `TASK-CA-0106` | Emisión desde proyecto principal | Proyecto principal | `pending` | Pendiente |
| 2 | `TASK-CA-0201` | Snapshot revisionado | Proyecto principal | `pending` | Pendiente |
| 2 | `TASK-CA-0202` | Cliente de snapshot y bootstrap | GoAnalytics | `pending` | Pendiente |
| 2 | `TASK-CA-0203` | Convergencia de identidades | GoAnalytics | `pending` | Pendiente |
| 2 | `TASK-CA-0204` | Certificar backfill inicial | Ambos proyectos | `pending` | Pendiente |
| 3 | `TASK-CA-0301` | Migración `007` y repositorios | GoAnalytics | `pending` | Pendiente |
| 3 | `TASK-CA-0302` | Proyector global incremental | GoAnalytics | `pending` | Pendiente |
| 3 | `TASK-CA-0303` | Programar `subject-jobs` | GoAnalytics | `pending` | Pendiente |
| 3 | `TASK-CA-0304` | Query API y ranking global | GoAnalytics | `pending` | Pendiente |
| 3 | `TASK-CA-0305` | Consumir sugerencias globales | Proyecto principal | `pending` | Pendiente |
| 4 | `TASK-CA-0401` | Publicar registro de contextos | Proyecto principal | `pending` | Pendiente |
| 4 | `TASK-CA-0402` | Migración `008`, registro y caché | GoAnalytics | `pending` | Pendiente |
| 4 | `TASK-CA-0403` | Resolver contextos al normalizar | GoAnalytics | `pending` | Pendiente |
| 4 | `TASK-CA-0404` | Resolver históricos iniciales | GoAnalytics | `pending` | Pendiente |
| 4 | `TASK-CA-0405` | Proyección contextual | GoAnalytics | `pending` | Pendiente |
| 4 | `TASK-CA-0406` | Ranking contextual y fallback | GoAnalytics | `pending` | Pendiente |
| 4 | `TASK-CA-0407` | Integración contextual principal | Proyecto principal | `pending` | Pendiente |
| 4 | `TASK-CA-0408` | Certificar MVP | Ambos proyectos | `pending` | Pendiente |

## 3. Gates De Fase

| Gate | Condición Para Aprobar | Estado Inicial |
|---|---|---|
| Fase 0 → 1 | Pendientes recuperables, conflictos aislados, migración `005`, resolver aditivo y flags seguros | `pending` |
| Fase 1 → 2 | Endpoint interno, identidad, normalización, lifecycle y emisor post-commit aceptados | `pending` |
| Fase 2 → 3 | Snapshot estable, bootstrap reanudable y backfill certificado | `pending` |
| Fase 3 → 4 | Proyección global, query API e integración global dentro de SLO | `pending` |
| Cierre MVP | Registro, resolución inicial, ranking contextual, rollout y regresión completa aceptados | `pending` |

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
