# Modelo De Datos Y Migraciones

## Convenciones

- PostgreSQL 16 y `BIGSERIAL` para claves internas.
- Timestamps en `TIMESTAMPTZ`; fecha analítica en `DATE`.
- IDs externos en `TEXT` y tratados como opacos.
- Estados en `TEXT` con `CHECK` para el contrato MVP.
- Índices de consulta comienzan por `tenant_id`.
- Todas las migraciones tienen `up` y `down` simétricos.
- Los `down` no se ejecutan en producción salvo aprobación explícita de pérdida de datos.

## Migración 005: Origen E Idempotencia De Eventos

Archivos previstos:

```text
migrations/postgres/005_event_source_and_persistence_outcomes.up.sql
migrations/postgres/005_event_source_and_persistence_outcomes.down.sql
```

Cambios:

```sql
ALTER TABLE analytics_events
    ADD COLUMN event_source TEXT NOT NULL DEFAULT 'web';

ALTER TABLE analytics_events
    ADD CONSTRAINT ck_analytics_events_event_source
    CHECK (event_source IN ('web', 'server'));
```

No se cambian los índices únicos actuales. El repositorio debe usar `INSERT ... ON CONFLICT DO NOTHING RETURNING id` y clasificar el conflicto consultando por `event_id`, `logical_event_id` e `idempotency_key`. Esa clasificación pertenece al adaptador, no requiere otra tabla.

## Migración 006: Sujetos, Detalles Y Bootstrap

Archivos previstos:

```text
migrations/postgres/006_create_analytics_subjects.up.sql
migrations/postgres/006_create_analytics_subjects.down.sql
```

### `analytics_subjects`

```text
id BIGSERIAL PRIMARY KEY
tenant_id TEXT NOT NULL
subject_type TEXT NOT NULL
external_subject_id TEXT NOT NULL
lifecycle_status TEXT NOT NULL DEFAULT 'unknown'
lifecycle_source_updated_at TIMESTAMPTZ
lifecycle_source_mutation_id TEXT
lifecycle_effective_at TIMESTAMPTZ
merged_into_subject_id BIGINT NULL FK analytics_subjects(id)
status_updated_at TIMESTAMPTZ NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Restricciones:

```sql
UNIQUE (tenant_id, subject_type, external_subject_id)
CHECK (subject_type = 'customer')
CHECK (lifecycle_status IN ('unknown', 'active', 'inactive', 'merged'))
CHECK ((lifecycle_status = 'merged') = (merged_into_subject_id IS NOT NULL))
CHECK (merged_into_subject_id IS NULL OR merged_into_subject_id <> id)
```

La aplicación valida que origen y destino de merge compartan tenant y tipo. PostgreSQL no puede expresar esa regla solamente con la FK simple.

Índices:

```text
ix_analytics_subjects_tenant_status
    (tenant_id, subject_type, lifecycle_status, external_subject_id)

ix_analytics_subjects_merged_into
    (merged_into_subject_id) WHERE merged_into_subject_id IS NOT NULL
```

### `analytics_event_subjects`

```text
id BIGSERIAL PRIMARY KEY
analytics_event_id BIGINT NOT NULL FK analytics_events(id) ON DELETE CASCADE
subject_ordinal INTEGER NOT NULL
client_event_id TEXT NOT NULL
logical_event_id TEXT
idempotency_key TEXT
tenant_id TEXT NOT NULL
site_id TEXT NOT NULL
event_source TEXT NOT NULL
event_name TEXT NOT NULL
event_time TIMESTAMPTZ NOT NULL
received_at TIMESTAMPTZ NOT NULL
analytics_subject_id BIGINT NULL FK analytics_subjects(id)
subject_type TEXT NOT NULL
external_subject_id TEXT NOT NULL
relation TEXT NOT NULL DEFAULT ''
operation_type TEXT
operation_id TEXT
context_definition_id BIGINT NULL
context_key TEXT
context_version INTEGER
claimed_context_family TEXT
resolved_context_family TEXT
context_resolution_status TEXT NOT NULL DEFAULT 'registry_disabled'
context_registry_revision_id BIGINT NULL
timezone_name TEXT
local_date DATE
normalization_status TEXT NOT NULL
normalization_reason TEXT
metadata JSONB NOT NULL DEFAULT '{}'::jsonb
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Estados de normalización:

```text
normalized
invalid_subject
unsupported_subject_type
timezone_unresolved
idempotency_conflict
lifecycle_ignored
```

Estados de contexto:

```text
registry_disabled
registered
unregistered
inactive_context
family_unresolved
```

Restricciones e índices:

```sql
UNIQUE (analytics_event_id, subject_ordinal)

UNIQUE (
    tenant_id,
    event_name,
    idempotency_key,
    subject_type,
    external_subject_id,
    relation
)
WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
```

```text
ix_event_subjects_watermark (updated_at, id)
ix_event_subjects_tenant_date (tenant_id, local_date, subject_type)
ix_event_subjects_subject_time (analytics_subject_id, event_time DESC)
ix_event_subjects_context_pending (context_resolution_status, id)
```

`subject_ordinal` es la posición original en `properties.subjects`. Duplicados exactos dentro del payload se registran una sola vez usando la primera posición.

### `analytics_subject_bootstrap`

```text
id BIGSERIAL PRIMARY KEY
subject_type TEXT NOT NULL
tenant_id TEXT NULL
status TEXT NOT NULL
cursor TEXT
source_revision TEXT
started_at TIMESTAMPTZ
completed_at TIMESTAMPTZ
processed_count BIGINT NOT NULL DEFAULT 0
failed_count BIGINT NOT NULL DEFAULT 0
last_error TEXT
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

`tenant_id` nulo representa el snapshot global. Solo puede existir una ejecución `running` por `subject_type`, garantizada por índice único parcial.

## Migración 007: Proyección Global

Archivos previstos:

```text
migrations/postgres/007_create_subject_global_projections.up.sql
migrations/postgres/007_create_subject_global_projections.down.sql
```

### `analytics_projection_runs`

```text
id BIGSERIAL PRIMARY KEY
projection_name TEXT NOT NULL
projection_version INTEGER NOT NULL
status TEXT NOT NULL
started_at TIMESTAMPTZ NOT NULL
completed_at TIMESTAMPTZ
source_from_updated_at TIMESTAMPTZ
source_from_id BIGINT
source_to_updated_at TIMESTAMPTZ
source_to_id BIGINT
affected_days INTEGER NOT NULL DEFAULT 0
rows_read BIGINT NOT NULL DEFAULT 0
rows_written BIGINT NOT NULL DEFAULT 0
last_error TEXT
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Estados: `running`, `completed`, `failed`, `skipped_locked`.

### `analytics_projection_checkpoints`

```text
projection_name TEXT NOT NULL
projection_version INTEGER NOT NULL
last_updated_at TIMESTAMPTZ NOT NULL
last_id BIGINT NOT NULL
updated_at TIMESTAMPTZ NOT NULL
PRIMARY KEY (projection_name, projection_version)
```

### `analytics_subject_tenant_daily`

```text
tenant_id TEXT NOT NULL
analytics_subject_id BIGINT NOT NULL FK analytics_subjects(id) ON DELETE CASCADE
local_date DATE NOT NULL
timezone_name TEXT NOT NULL
impression_count BIGINT NOT NULL DEFAULT 0
view_count BIGINT NOT NULL DEFAULT 0
selection_count BIGINT NOT NULL DEFAULT 0
association_count BIGINT NOT NULL DEFAULT 0
last_association_at TIMESTAMPTZ
projection_version INTEGER NOT NULL
computed_at TIMESTAMPTZ NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
PRIMARY KEY (tenant_id, analytics_subject_id, local_date, timezone_name, projection_version)
```

Índice de ranking:

```text
ix_subject_tenant_daily_ranking
    (tenant_id, local_date, association_count DESC, last_association_at DESC)
```

## Migración 008: Registro Y Proyección Contextual

Archivos previstos:

```text
migrations/postgres/008_create_subject_context_registry.up.sql
migrations/postgres/008_create_subject_context_registry.down.sql
```

### Registro

```text
analytics_context_registry_revisions
- id BIGSERIAL PRIMARY KEY
- schema_version INTEGER NOT NULL
- registry_version BIGINT NOT NULL
- registry_revision TEXT NOT NULL UNIQUE
- payload JSONB NOT NULL
- payload_checksum TEXT NOT NULL
- validation_status TEXT NOT NULL
- loaded_at TIMESTAMPTZ NOT NULL
- activated_at TIMESTAMPTZ
- created_at TIMESTAMPTZ NOT NULL DEFAULT now()

analytics_context_definitions
- id BIGSERIAL PRIMARY KEY
- context_key TEXT NOT NULL
- context_version INTEGER NOT NULL
- context_family TEXT NOT NULL
- parent_definition_id BIGINT NULL FK analytics_context_definitions(id)
- created_at TIMESTAMPTZ NOT NULL DEFAULT now()
- UNIQUE (context_key, context_version)

analytics_context_registry_entries
- registry_revision_id BIGINT NOT NULL FK analytics_context_registry_revisions(id) ON DELETE CASCADE
- context_definition_id BIGINT NOT NULL FK analytics_context_definitions(id)
- status TEXT NOT NULL
- PRIMARY KEY (registry_revision_id, context_definition_id)
```

`context_family` admite las siete familias del MVP. `status` admite `active` e `inactive`.

### Contextos No Registrados

```text
analytics_unregistered_contexts
- context_key TEXT NOT NULL
- context_version INTEGER NOT NULL
- claimed_context_family TEXT
- first_seen_at TIMESTAMPTZ NOT NULL
- last_seen_at TIMESTAMPTZ NOT NULL
- event_count BIGINT NOT NULL
- reconciliation_status TEXT NOT NULL
- last_reconciliation_attempt_at TIMESTAMPTZ
- last_error TEXT
- PRIMARY KEY (context_key, context_version)
```

### Agregado Contextual

```text
analytics_subject_context_daily
- tenant_id TEXT NOT NULL
- analytics_subject_id BIGINT NOT NULL FK analytics_subjects(id) ON DELETE CASCADE
- context_definition_id BIGINT NOT NULL FK analytics_context_definitions(id)
- local_date DATE NOT NULL
- timezone_name TEXT NOT NULL
- impression_count BIGINT NOT NULL DEFAULT 0
- view_count BIGINT NOT NULL DEFAULT 0
- selection_count BIGINT NOT NULL DEFAULT 0
- association_count BIGINT NOT NULL DEFAULT 0
- last_association_at TIMESTAMPTZ
- projection_version INTEGER NOT NULL
- computed_at TIMESTAMPTZ NOT NULL
- created_at TIMESTAMPTZ NOT NULL DEFAULT now()
- updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
- PRIMARY KEY (
    tenant_id,
    analytics_subject_id,
    context_definition_id,
    local_date,
    timezone_name,
    projection_version
  )
```

Índice de ranking:

```text
ix_subject_context_daily_ranking
    (tenant_id, context_definition_id, local_date,
     association_count DESC, last_association_at DESC)
```

La migración agrega FKs de `analytics_event_subjects.context_definition_id` y `context_registry_revision_id`, creadas como columnas nullable en `006` para permitir el despliegue incremental.

## Política De Escritura

- Sujeto, evento crudo y detalle normalizado se guardan en la misma transacción por evento.
- Los agregados nunca se actualizan en el camino crítico del worker.
- Cada día afectado se recalcula desde `analytics_event_subjects`; no se aplican incrementos acumulativos.
- Los repositorios devuelven resultados `inserted`, `duplicate_exact`, `duplicate_logical`, `duplicate_idempotency` o `conflict` por evento.
- Los estados semánticos fallidos se persisten; los errores de PostgreSQL desconocidos abortan y dejan el mensaje pendiente.

## Orden De Down

`008` elimina primero FKs contextuales, agregado, pendientes, entries, definiciones y revisiones. `007` elimina agregado global, checkpoints y runs. `006` elimina bootstrap, detalles y sujetos. `005` elimina constraint y `event_source`.
