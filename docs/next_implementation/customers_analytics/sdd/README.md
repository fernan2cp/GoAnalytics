# SDD MVP De Estadísticas De Clientes

## Propósito

Este paquete convierte la estrategia de estadísticas de clientes en una especificación implementable para GoAnalytics y para el proyecto principal que consumirá la capacidad.

El MVP cubre las Fases 0 a 4 de la guía: confiabilidad, contratos, identidad de sujetos, lifecycle, backfill, elegibilidad, proyección diaria, ranking global, registro de contextos y ranking contextual con fallback global.

Todos los componentes nuevos son opt-in. El tracking público actual debe conservar contratos y comportamiento cuando las capacidades de sujetos estén deshabilitadas.

## Fuente

- `docs/next_implementation/customers_analytics/estrategia_estadisticas_clientes_goanalytics.md`.
- Código actual de `services/ingest`, `services/worker`, `packages/web-sdk` y `migrations/postgres`.
- Contratos vigentes en `docs/event-contract.md`, `docs/event-conventions.md` y `docs/resolver-contract.md`.

Las decisiones cerradas en este SDD prevalecen para el MVP cuando la guía todavía presenta una alternativa como pendiente.

## Mapa De Documentos

- [`00_baseline_fase_0.md`](00_baseline_fase_0.md): estado actual, capacidades reutilizables, deuda bloqueante y decisiones cerradas.
- [`01_requisitos.md`](01_requisitos.md): requisitos funcionales y no funcionales con IDs estables.
- [`02_modelo_datos_y_migraciones.md`](02_modelo_datos_y_migraciones.md): tablas, restricciones, índices y migraciones `005` a `008`.
- [`03_contratos_eventos_y_resolvers.md`](03_contratos_eventos_y_resolvers.md): contratos HTTP, eventos, resolver, snapshot y registro.
- [`04_confiabilidad_worker_y_normalizacion.md`](04_confiabilidad_worker_y_normalizacion.md): recuperación Redis, persistencia idempotente y normalización.
- [`05_lifecycle_backfill_y_elegibilidad.md`](05_lifecycle_backfill_y_elegibilidad.md): orden temporal, estados, backfill y concurrencia.
- [`06_proyecciones_y_agregados.md`](06_proyecciones_y_agregados.md): proyección diaria, watermark, recomputación y rankings.
- [`07_registro_contextos.md`](07_registro_contextos.md): publicación, validación, persistencia, caché y resolución.
- [`08_api_sugerencias_y_seguridad.md`](08_api_sugerencias_y_seguridad.md): consulta, autenticación, aislamiento, fallback y frescura.
- [`09_integracion_proyecto_principal.md`](09_integracion_proyecto_principal.md): responsabilidades end-to-end del proyecto principal.
- [`10_observabilidad_operacion_y_alertas.md`](10_observabilidad_operacion_y_alertas.md): métricas, logs, health checks y runbooks.
- [`11_plan_pruebas.md`](11_plan_pruebas.md): escenarios y comandos de validación.
- [`12_rollout_y_rollback.md`](12_rollout_y_rollback.md): activación progresiva y reversión segura.
- [`13_plan_de_tareas.md`](13_plan_de_tareas.md): tareas de las Fases 0 a 4 y correcciones de cierre `TASK-CA-0501` a `TASK-CA-0506`.
- [`14_criterios_de_aceptacion.md`](14_criterios_de_aceptacion.md): condiciones verificables de salida.
- [`15_estado_implementacion.md`](15_estado_implementacion.md): estado actual y evidencia de certificación, distinguiendo el historial de agosto de la recertificación de noviembre.

## Regla De Trazabilidad

- Requisitos: `REQ-CA-*`.
- Tareas: `TASK-CA-*`.
- Criterios: `AC-CA-*`.

Cada tarea referencia al menos un requisito y un criterio. Cada requisito aparece en una tarea y en un criterio. Cada criterio tiene validación objetiva en `11_plan_pruebas.md`.

## Alcance Del MVP

- Recuperación de mensajes pendientes y tratamiento de conflictos idempotentes.
- Endpoint interno de eventos server-to-server.
- Metadata de zona horaria IANA.
- Identidad tenant-global de clientes.
- Eventos de interacción, asociación y lifecycle.
- Backfill de activos, inactivos y fusionados.
- Proyección diaria global y contextual.
- API interna de sugerencias con ranking determinista.
- Registro global de contextos con siete familias genéricas.
- Integración del proyecto principal para emitir eventos, publicar fuentes y consumir sugerencias.
- Procesos separados `subject-jobs` y `query-api` dentro del módulo worker.

## Fuera Del Alcance

- Conciliación general posterior al arranque del MVP.
- Scoring ponderado o mezcla de scores.
- Fallback por familia y roll-up jerárquico.
- Transferencia de conteos por merge.
- Corrección o reversión de asociaciones históricas.
- Correlación de sujetos entre tenants.
- Datos personales en GoAnalytics.
- Scopes multiproyecto, UI administrativa o acceso directo a tablas.
- Outbox durable en el proyecto principal.

## Estado Y Validación

Las tareas nuevas comienzan en `pending`; solo la evidencia aceptada permite cerrarlas. El estado vigente, los commits de ambos repositorios y la certificación cruzada se registran en `15_estado_implementacion.md`. `11_plan_pruebas.md` contiene los comandos reproducibles. La preparación para producción está resumida en `../instructivo_configuracion_mvp_produccion.md`; certificar el MVP no equivale a desplegarlo ni activar tenants de negocio.
