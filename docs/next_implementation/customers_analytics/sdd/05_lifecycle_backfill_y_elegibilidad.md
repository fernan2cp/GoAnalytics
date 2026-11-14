# Lifecycle, Backfill Y Elegibilidad

## Orden Temporal Compuesto

El MVP no exige una secuencia numérica nativa. Cada mutación entrega:

```text
source_updated_at TIMESTAMPTZ
source_mutation_id TEXT
```

La tupla entrante es nueva cuando:

```text
incoming.source_updated_at > current.source_updated_at
OR
incoming.source_updated_at = current.source_updated_at
AND incoming.source_mutation_id > current.source_mutation_id
```

La comparación del desempate es binaria y lexicográfica sobre UTF-8. El proyecto principal debe generar un ID estable para la misma mutación. `effective_at` describe cuándo tuvo efecto en negocio, pero no ordena actualizaciones.

DragonFullAV persiste esa tupla en `analytics_customer_lifecycle` (revisión tenant `custanalytics01`). Captura SQL opt-in mediante `analytics_customer_lifecycle_configure(true)`, apagada por defecto. La misma fila alimenta snapshot y emisión post-commit; leer un snapshot no crea una versión nueva. Altas, bajas y conversiones actualizan elegibilidad en la transacción de negocio. Una fusión debe escribir estado/destino en ese registro dentro de su transacción; el trigger asigna la nueva tupla. El MVP no agrega un flujo de negocio de fusión inexistente.

Un evento igual o anterior produce `lifecycle_ignored`, log informativo y no cambia el sujeto.

## Transiciones

| Evento | Estado destino | Regla adicional |
| --- | --- | --- |
| `subject_deactivated` | `inactive` | Limpia `merged_into_subject_id`. |
| `subject_reactivated` | `active` | Limpia `merged_into_subject_id`. |
| `subject_merged` | `merged` | Exige destino existente o creado y distinto del origen. |

No se restringen transiciones por estado anterior si la tupla es nueva. Esto permite reparar estados perdidos. La auditoría conserva evento crudo y detalle normalizado.

## Elegibilidad

La API incluye solo sujetos `active` al momento de la consulta. No se eliminan agregados al cambiar estado; el filtro se aplica contra `analytics_subjects`.

El proyecto principal debe aplicar después disponibilidad, permisos, restricciones comerciales y existencia real del cliente.

## Cliente De Snapshot

El módulo worker incorpora un puerto `SubjectSnapshotResolver` y adaptador HTTP con:

```env
SUBJECT_SNAPSHOT_URL=http://backend/api/v1/internal/analytics/subjects/snapshot
SUBJECT_SOURCE_TOKEN=change_me
SUBJECT_SNAPSHOT_TIMEOUT_MS=5000
SUBJECT_BOOTSTRAP_PAGE_SIZE=500
```

El adaptador valida status HTTP, forma JSON, `source_revision`, cursor y ausencia de PII no permitida por contrato.

## Ejecución Del Backfill

Comando conceptual:

```powershell
subject-jobs --mode bootstrap
subject-jobs --mode bootstrap --force-new-snapshot
```

Algoritmo:

1. Crear o retomar una fila `analytics_subject_bootstrap`.
2. Si no existe revisión, solicitar primera página y fijarla.
3. Procesar páginas de hasta 500 sujetos.
4. Upsert de cada sujeto con la misma regla temporal de lifecycle.
5. Confirmar cursor y conteos en la misma transacción de la página.
6. Repetir hasta `has_more=false`.
7. Marcar `completed` y registrar revisión.

Una interrupción reanuda desde el último cursor confirmado. Una ejecución completada no vuelve a correr automáticamente; requiere `--force-new-snapshot`.

## Revisión Expirada

Ante `409 snapshot_revision_expired`:

- marcar ejecución `failed`;
- conservar cursor, revisión y error;
- no mezclar páginas de revisiones distintas;
- requerir `--force-new-snapshot` para comenzar otra ejecución desde cero;
- los upserts ya aplicados son seguros porque usan orden temporal compuesto.

## Concurrencia

- La unicidad tenant-global converge backfill y lazy creation.
- El upsert bloquea la fila del sujeto antes de comparar lifecycle.
- Dos páginas o eventos concurrentes no pueden reducir la tupla almacenada.
- El índice parcial permite un solo bootstrap `running` por `subject_type`.
- Un advisory lock evita dos procesos bootstrap simultáneos.

## Merge En Backfill

Los sujetos de la página se insertan primero sin FK de merge y después se resuelven destinos dentro de la misma transacción. Si el destino no está en la página ni existe, se crea como `unknown` con su ID externo y luego se aplica el merge al origen.

No se copian conteos. La consulta excluye el origen merged.

## Recuperación Por Pérdida De Retry Directo

El retry directo puede perder lifecycle. El procedimiento MVP es ejecutar un nuevo snapshot completo con `--force-new-snapshot`. Las asociaciones perdidas no se reconstruyen por backfill; se documenta como riesgo aceptado hasta una futura outbox o conciliación.
