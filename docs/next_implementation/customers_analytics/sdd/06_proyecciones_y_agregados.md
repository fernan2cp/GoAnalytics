# Proyecciones Y Agregados

## Proceso

El binario `cmd/subject-jobs` admite:

```text
serve
bootstrap
project-global
resolve-contexts
project-context
```

`serve` mantiene health HTTP y evalúa cada 60 segundos si la proyección diaria está vencida. No ejecuta bootstrap automáticamente.

## Configuración

```env
SUBJECT_PROJECTION_ENABLED=false
SUBJECT_PROJECTION_INTERVAL_SECONDS=86400
SUBJECT_PROJECTION_STALE_AFTER_SECONDS=172800
SUBJECT_PROJECTION_BATCH_SIZE=5000
SUBJECT_PROJECTION_VERSION=1
SUBJECT_JOBS_HEALTH_HTTP_HOST=0.0.0.0
SUBJECT_JOBS_HEALTH_HTTP_PORT=8080
```

La primera ejecución corre al iniciar si no existe un run completado. Las siguientes corren cuando transcurrieron 86.400 segundos desde el último éxito.

## Exclusión Mutua

Cada proyección adquiere un advisory lock PostgreSQL derivado de `projection_name + projection_version`. Si no lo obtiene:

- registra run `skipped_locked`;
- no modifica checkpoint ni agregados;
- termina sin error operativo.

El lock se mantiene en una conexión dedicada durante toda la ejecución y se libera al cerrar la conexión, incluso ante cancelación.

## Watermark

Orden total:

```text
analytics_event_subjects.updated_at ASC,
analytics_event_subjects.id ASC
```

El checkpoint conserva ambos valores. La consulta usa:

```text
updated_at > last_updated_at
OR updated_at = last_updated_at AND id > last_id
```

Solo filas `normalization_status=normalized`, con sujeto y `local_date`, participan.

## Recomputación Global

1. Leer cambios por lotes de 5.000.
2. Reunir claves afectadas `tenant_id + analytics_subject_id + local_date + timezone_name`.
3. Para cada día afectado, recalcular desde todas las filas normalizadas de esa clave.
4. Reemplazar el agregado con upsert absoluto, no incrementos.
5. Eliminar la fila agregada si el nuevo total de todas las señales es cero.
6. Actualizar checkpoint solo después de escribir todas las claves del rango.

Un evento tardío de cualquier antigüedad se detecta por `updated_at` y recalcula su `local_date`; no depende de una ventana temporal fija.

## Recomputación Contextual

Usa el mismo watermark con nombre de proyección distinto. Solo incluye:

```text
context_resolution_status = registered
context_definition_id IS NOT NULL
```

La clave agrega `context_definition_id`. Contextos desconocidos, inactivos o con familia no resuelta no crean filas contextuales.

## Mapeo De Señales

| Evento | Contador |
| --- | --- |
| `subject_impression` | `impression_count` |
| `subject_viewed` | `view_count` |
| `subject_selected` | `selection_count` |
| `subject_associated` | `association_count` y máximo `last_association_at` |

Lifecycle no incrementa contadores.

## Ventanas De Consulta

```env
SUBJECT_SUGGESTION_CONTEXT_WINDOW_DAYS=30
SUBJECT_SUGGESTION_GLOBAL_WINDOW_DAYS=90
```

Las ventanas son inclusivas desde la fecha local actual del tenant. La API suma filas diarias dentro del rango. El resolver garantiza una zona canónica por tenant; si hubo cambio, cada fila conserva la zona aplicada y no se reescribe historia.

## Ranking

```text
association_count DESC
last_association_at DESC NULLS LAST
external_subject_id ASC
```

La consulta agrega primero por sujeto dentro de la ventana y aplica el orden después de sumar días.

## Atomicidad Y Fallos

- Cada lote de claves afectadas y su avance se confirman en una transacción.
- Un fallo deja checkpoint anterior y permite recomputar el lote.
- El run termina `failed` con error sanitizado.
- Una ejecución fallida no invalida la última proyección válida.
- Readiness del job depende de PostgreSQL; Redis solo se requiere para operaciones de registro que lo usen.

## Objetivos MVP

- Proyección completa menor o igual a 30 minutos con 500.000 eventos mensuales.
- Ausencia de doble conteo después de repetir una ejecución.
- Capacidad de retomar después de cancelación sin saltar filas.
- Máximo una ejecución activa por proyección.
