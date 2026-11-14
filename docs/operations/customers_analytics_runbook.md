# Operación De Estadísticas De Clientes

## Endpoints

- `/health` solo confirma que el proceso está vivo.
- `/ready` confirma las dependencias requeridas por ese proceso.
- `/metrics` expone el catálogo Prometheus sin tenant, subject ID, context key ni event ID como labels.

Nunca copiar tokens, DSN, payloads crudos ni identificadores personales en logs o tickets. Los IDs técnicos de tenant/site y los conteos agregados sí pueden usarse para diagnóstico.

## Pendientes Redis

1. Consultar el PEL, consumidores e idle sin ejecutar `XDEL`.
2. Confirmar incrementos de `redis_autoclaim_total` y el último error estable.
3. Si aumentó `redis_retry_exhausted_total`, revisar el rechazo durable antes de corregir la fuente.
4. Restaurar PostgreSQL/Redis o corregir el payload y permitir que el ciclo normal confirme con `XACK`.

## Proyección Fallida O Vencida

1. Revisar el último run, checkpoint, advisory lock y disponibilidad PostgreSQL.
2. Ejecutar manualmente `subject-jobs project-global` o `project-context` cuando esos comandos estén desplegados.
3. Confirmar un run `completed` y lag menor a 48 horas antes de retirar el incidente.
4. Para rollback, detener jobs/query API o apagar sus flags; no eliminar agregados.

## Registro Inválido

1. Mantener la última revisión válida activa.
2. Comparar revisión, checksum y errores estables sin registrar el token.
3. Corregir la fuente y publicar una revisión nueva; nunca mutar una revisión existente.

## Backfill Detenido

1. Revisar cursor, `source_revision`, conteos y último error.
2. Reanudar el mismo bootstrap si la revisión sigue vigente.
3. Ante `snapshot_revision_expired`, iniciar un snapshot nuevo y conservar la ejecución anterior para auditoría.

## Evidencia De Incidente

Registrar hora UTC, proceso, versión, flag relevante, operación, resultado, duración, métricas y error estable. No adjuntar payloads completos. La reversión productiva es operativa mediante flags y detención de binarios; los `down` requieren autorización explícita de pérdida de datos.
