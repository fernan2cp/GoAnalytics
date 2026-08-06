# Requisitos

## Compatibilidad Y Activación

- `REQ-CA-001`: `POST /v1/events` debe mantener autenticación, validación, aliases, payload y respuesta actuales.
- `REQ-CA-002`: Todas las capacidades de sujetos deben estar desactivadas por defecto mediante flags independientes.
- `REQ-CA-003`: Las migraciones deben ser aditivas y no deben eliminar ni reinterpretar datos actuales.
- `REQ-CA-004`: Un error semántico de sujeto no debe impedir persistir el evento crudo válido.
- `REQ-CA-005`: La suite existente de ingest, worker y SDK debe continuar pasando.

## Confiabilidad Del Stream

- `REQ-CA-006`: El worker debe reclamar periódicamente mensajes pendientes con idle time vencido.
- `REQ-CA-007`: Cada mensaje debe mapearse, procesarse y confirmarse de forma independiente dentro de un batch leído.
- `REQ-CA-008`: Un mensaje inválido debe persistirse como rechazo antes de confirmarse.
- `REQ-CA-009`: Un conflicto idempotente esperado no debe revertir eventos no relacionados.
- `REQ-CA-010`: La confirmación Redis solo debe ocurrir después de persistencia o rechazo durable.

## Eventos Y Resolución

- `REQ-CA-011`: Debe existir `POST /v1/internal/events` con Bearer dedicado y respuesta `202`.
- `REQ-CA-012`: El evento interno debe transportar `event_source=server` y no requerir navegación o sesión.
- `REQ-CA-013`: El tenant de eventos internos debe derivarse de `site_code` mediante resolver confiable.
- `REQ-CA-014`: El resolver debe aceptar `source` opcional y devolver una zona horaria IANA válida.
- `REQ-CA-015`: Eventos públicos existentes deben persistir `event_source=web` sin que el consumidor lo envíe.
- `REQ-CA-016`: El contrato canónico debe usar `properties.subjects` y permitir uno o varios sujetos.
- `REQ-CA-017`: Los eventos soportados son impression, viewed, selected, associated, deactivated, reactivated y merged.

## Identidad, Lifecycle Y Elegibilidad

- `REQ-CA-018`: La identidad debe ser única por `tenant_id + subject_type + external_subject_id`.
- `REQ-CA-019`: El MVP debe rechazar normalización para tipos distintos de `customer` sin rechazar el evento crudo.
- `REQ-CA-020`: Los estados deben ser `unknown`, `active`, `inactive` y `merged`; solo `active` es elegible.
- `REQ-CA-021`: Interacciones web crean sujetos `unknown`; asociaciones internas pueden crearlos `active`.
- `REQ-CA-022`: Lifecycle debe aplicar solo si la tupla `source_updated_at + source_mutation_id` es mayor que la almacenada.
- `REQ-CA-023`: Un lifecycle antiguo debe ser no-op auditable.
- `REQ-CA-024`: Merge debe conservar historia y no transferir conteos durante el MVP.
- `REQ-CA-025`: La idempotencia de asociaciones debe tener alcance tenant aunque intervengan sites diferentes.

## Backfill

- `REQ-CA-026`: El proyecto principal debe exponer un snapshot paginado sin datos personales.
- `REQ-CA-027`: La primera página debe fijar `source_revision`; las siguientes deben usarla.
- `REQ-CA-028`: Una revisión expirada debe responder `409 snapshot_revision_expired`.
- `REQ-CA-029`: El backfill debe incluir activos, inactivos y fusionados.
- `REQ-CA-030`: El job debe ser reanudable, idempotente y concurrente con lazy creation.
- `REQ-CA-031`: El backfill debe conservar cursor, revisión, conteos, estado y último error.

## Proyección Global

- `REQ-CA-032`: La proyección debe ejecutarse fuera del worker crítico en un binario separado.
- `REQ-CA-033`: Debe correr al iniciar si está vencida y luego como máximo una vez cada 86.400 segundos.
- `REQ-CA-034`: Un advisory lock PostgreSQL debe impedir ejecuciones concurrentes.
- `REQ-CA-035`: El watermark debe ordenar por `updated_at + id`.
- `REQ-CA-036`: Los días afectados deben recalcularse completos y reemplazarse atómicamente.
- `REQ-CA-037`: Los conteos deben separar impression, view, selection y association.
- `REQ-CA-038`: El agregado debe conservar `last_association_at`, zona horaria, versión y `computed_at`.
- `REQ-CA-039`: La señal de ranking del MVP debe ser `association_count`.

## Registro Y Contexto

- `REQ-CA-040`: El proyecto principal debe publicar un registro global revisionado.
- `REQ-CA-041`: Las familias permitidas deben ser las siete definidas por el MVP.
- `REQ-CA-042`: Una definición debe ser inmutable por `context_key + context_version`.
- `REQ-CA-043`: El estado activo o inactivo debe pertenecer a una revisión concreta.
- `REQ-CA-044`: El orden de carga debe ser memoria, Redis, PostgreSQL y endpoint fuente.
- `REQ-CA-045`: La rehidratación debe usar lock Redis con token y liberación segura.
- `REQ-CA-046`: Una revisión inválida no debe reemplazar la última válida.
- `REQ-CA-047`: La resolución debe persistir definición, revisión y estado usados.
- `REQ-CA-048`: Contextos desconocidos deben excluirse del agregado contextual y conservar contribución global.
- `REQ-CA-049`: La Fase 4 debe resolver idempotentemente filas históricas creadas con registro deshabilitado.

## Consulta Y Seguridad

- `REQ-CA-050`: Debe existir un binario query API separado con health, readiness y métricas.
- `REQ-CA-051`: `GET /v1/internal/subjects/suggestions` debe exigir Bearer dedicado.
- `REQ-CA-052`: `tenant_id` y `subject_type` deben ser obligatorios; `context_version` acompaña a `context_key`.
- `REQ-CA-053`: `limit` debe usar default 10 y máximo 100.
- `REQ-CA-054`: El ranking debe ordenar por conteo, último uso e ID externo.
- `REQ-CA-055`: El fallback permitido debe ser contexto exacto a global, sin mezcla de scores.
- `REQ-CA-056`: La respuesta no debe exponer IDs internos, datos personales ni payload crudo.
- `REQ-CA-057`: Sin datos debe responder `200`; sin proyección debe responder `503 projection_unavailable`.
- `REQ-CA-058`: Una proyección mayor a 48 horas debe marcarse stale o rechazarse según configuración.
- `REQ-CA-059`: Toda consulta debe aislar datos por tenant en handler, caso de uso y repositorio.

## Proyecto Principal Y Operación

- `REQ-CA-060`: Asociaciones y lifecycle deben emitirse después del commit con clave idempotente estable.
- `REQ-CA-061`: El emisor debe realizar tres intentos directos con timeout y backoff, sin revertir el negocio.
- `REQ-CA-062`: El proyecto principal debe recuperar los datos reales del cliente a partir del ID sugerido.
- `REQ-CA-063`: El proyecto principal debe aplicar elegibilidad comercial adicional.
- `REQ-CA-064`: Worker, jobs y query API deben emitir métricas y logs estructurados sin secretos.
- `REQ-CA-065`: Rollout y rollback deben controlarse mediante flags, sin borrar tablas en producción.
- `REQ-CA-066`: El MVP debe cumplir los objetivos de volumen y latencia definidos en este SDD.
