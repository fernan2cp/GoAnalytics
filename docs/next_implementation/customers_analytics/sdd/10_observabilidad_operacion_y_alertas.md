# Observabilidad, Operación Y Alertas

## Endpoints Operativos

Worker, `subject-jobs` y `query-api` exponen:

- `/health`: proceso vivo.
- `/ready`: dependencias requeridas disponibles.
- `/metrics`: métricas Prometheus.

La implementación agrega un puerto de métricas en application y adaptadores no-op/Prometheus en bootstrap. Domain no importa Prometheus.

## Métricas Mínimas

```text
redis_pending_events_count
redis_autoclaim_total
redis_retry_exhausted_total
event_persistence_outcomes_total{outcome}
subject_normalization_total{status,event_name}
subject_lifecycle_ignored_total{reason}
subject_bootstrap_pages_total{status}
subject_bootstrap_subjects_total{status}
subject_bootstrap_duration_seconds
subject_projection_runs_total{projection,status}
subject_projection_duration_seconds{projection}
subject_projection_lag_seconds{projection}
subject_projection_rows_total{projection,operation}
subject_context_registry_loads_total{status}
subject_context_registry_revision_info{revision}
subject_unregistered_contexts_total
subject_query_requests_total{status,source_level}
subject_query_duration_seconds{status}
subject_query_stale_total
```

No usar tenant, subject ID, context key ni event ID como labels para evitar cardinalidad y fuga.

## Logs

Todos los procesos incluyen `request_id` o `run_id`, componente, operación, resultado, duración y error estable. Los logs no contienen tokens, payload completo, IDs personales ni DSN.

Campos de diagnóstico permitidos: tenant ID técnico, site ID, event name, registry revision, projection version y conteos.

## Alertas Recomendadas

| Alerta | Condición inicial |
| --- | --- |
| Pendientes Redis | Mensajes con idle mayor a 5 minutos. |
| Retry agotado | Cualquier incremento en 15 minutos. |
| Proyección fallida | Último run `failed`. |
| Proyección stale | Lag mayor a 48 horas. |
| Registro ausente | Contextos habilitados sin revisión válida. |
| Registro inválido | Carga inválida en dos intentos consecutivos. |
| Backfill detenido | `running` sin avance durante 15 minutos. |
| Query degradada | p95 mayor a 300 ms durante 15 minutos. |
| Error de query | 5xx mayor a 1% durante 15 minutos. |

## Runbooks

### Pendientes Redis

1. Revisar PEL, consumidores e idle.
2. Confirmar ejecución de autoclaim.
3. Inspeccionar último error estable.
4. No borrar mensajes manualmente.
5. Corregir dependencia y permitir reintento.

### Proyección Fallida O Stale

1. Confirmar PostgreSQL y advisory lock.
2. Revisar último run y checkpoint.
3. Ejecutar `subject-jobs project-global` o `project-context` manualmente.
4. Verificar nuevo `completed` antes de desactivar stale.

### Registro Inválido

1. Mantener última revisión válida.
2. Comparar revisión, checksum y errores de validación.
3. Corregir el archivo fuente.
4. Publicar una nueva revisión, no reutilizar la inválida.

### Retry Directo Perdido

Lifecycle: ejecutar un snapshot nuevo. Asociación: registrar incidente analítico; no insertar SQL manual ni fabricar eventos sin identidad estable.

## Retención

Runs, bootstrap y revisiones son auditoría operativa. El MVP no borra automáticamente estas filas. Una política de retención se evaluará después de medir volumen.
