CREATE TABLE IF NOT EXISTS analytics_subjects (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    external_subject_id TEXT NOT NULL,
    lifecycle_status TEXT NOT NULL DEFAULT 'unknown',
    lifecycle_source_updated_at TIMESTAMPTZ,
    lifecycle_source_mutation_id TEXT,
    lifecycle_effective_at TIMESTAMPTZ,
    merged_into_subject_id BIGINT REFERENCES analytics_subjects(id),
    status_updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ux_analytics_subjects_identity UNIQUE (tenant_id, subject_type, external_subject_id),
    CONSTRAINT ck_analytics_subjects_subject_type CHECK (subject_type = 'customer'),
    CONSTRAINT ck_analytics_subjects_lifecycle_status CHECK (lifecycle_status IN ('unknown', 'active', 'inactive', 'merged')),
    CONSTRAINT ck_analytics_subjects_merge_target CHECK ((lifecycle_status = 'merged') = (merged_into_subject_id IS NOT NULL)),
    CONSTRAINT ck_analytics_subjects_distinct_merge_target CHECK (merged_into_subject_id IS NULL OR merged_into_subject_id <> id),
    CONSTRAINT ck_analytics_subjects_lifecycle_order_pair CHECK (
        (lifecycle_source_updated_at IS NULL AND lifecycle_source_mutation_id IS NULL)
        OR (lifecycle_source_updated_at IS NOT NULL AND lifecycle_source_mutation_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS ix_analytics_subjects_tenant_status
ON analytics_subjects(tenant_id, subject_type, lifecycle_status, external_subject_id);

CREATE INDEX IF NOT EXISTS ix_analytics_subjects_merged_into
ON analytics_subjects(merged_into_subject_id) WHERE merged_into_subject_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS analytics_event_subjects (
    id BIGSERIAL PRIMARY KEY,
    analytics_event_id BIGINT NOT NULL REFERENCES analytics_events(id) ON DELETE CASCADE,
    subject_ordinal INTEGER NOT NULL,
    client_event_id TEXT NOT NULL,
    logical_event_id TEXT,
    idempotency_key TEXT,
    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    event_source TEXT NOT NULL,
    event_name TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    analytics_subject_id BIGINT REFERENCES analytics_subjects(id),
    subject_type TEXT NOT NULL,
    external_subject_id TEXT NOT NULL,
    relation TEXT NOT NULL DEFAULT '',
    operation_type TEXT,
    operation_id TEXT,
    context_definition_id BIGINT,
    context_key TEXT,
    context_version INTEGER,
    claimed_context_family TEXT,
    resolved_context_family TEXT,
    context_resolution_status TEXT NOT NULL DEFAULT 'registry_disabled',
    context_registry_revision_id BIGINT,
    timezone_name TEXT,
    local_date DATE,
    normalization_status TEXT NOT NULL,
    normalization_reason TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ux_analytics_event_subjects_ordinal UNIQUE (analytics_event_id, subject_ordinal),
    CONSTRAINT ck_analytics_event_subjects_ordinal CHECK (subject_ordinal >= 0),
    CONSTRAINT ck_analytics_event_subjects_event_source CHECK (event_source IN ('web', 'server')),
    CONSTRAINT ck_analytics_event_subjects_normalization_status CHECK (
        normalization_status IN ('normalized', 'invalid_subject', 'unsupported_subject_type', 'timezone_unresolved', 'idempotency_conflict', 'lifecycle_ignored')
    ),
    CONSTRAINT ck_analytics_event_subjects_context_resolution_status CHECK (
        context_resolution_status IN ('registry_disabled', 'registered', 'unregistered', 'inactive_context', 'family_unresolved')
    ),
    CONSTRAINT ck_analytics_event_subjects_context_version CHECK (context_version IS NULL OR context_version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_event_subjects_tenant_idempotency
ON analytics_event_subjects(tenant_id, event_name, idempotency_key, subject_type, external_subject_id, relation)
WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

CREATE INDEX IF NOT EXISTS ix_event_subjects_watermark ON analytics_event_subjects(updated_at, id);
CREATE INDEX IF NOT EXISTS ix_event_subjects_tenant_date ON analytics_event_subjects(tenant_id, local_date, subject_type);
CREATE INDEX IF NOT EXISTS ix_event_subjects_subject_time ON analytics_event_subjects(analytics_subject_id, event_time DESC);
CREATE INDEX IF NOT EXISTS ix_event_subjects_context_pending ON analytics_event_subjects(context_resolution_status, id);

CREATE TABLE IF NOT EXISTS analytics_subject_bootstrap (
    id BIGSERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    tenant_id TEXT,
    status TEXT NOT NULL,
    cursor TEXT,
    source_revision TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    processed_count BIGINT NOT NULL DEFAULT 0,
    failed_count BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_analytics_subject_bootstrap_subject_type CHECK (subject_type = 'customer'),
    CONSTRAINT ck_analytics_subject_bootstrap_status CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    CONSTRAINT ck_analytics_subject_bootstrap_counts CHECK (processed_count >= 0 AND failed_count >= 0),
    CONSTRAINT ck_analytics_subject_bootstrap_completion CHECK (completed_at IS NULL OR status IN ('completed', 'failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_analytics_subject_bootstrap_running
ON analytics_subject_bootstrap(subject_type) WHERE status = 'running';

CREATE INDEX IF NOT EXISTS ix_analytics_subject_bootstrap_tenant_status
ON analytics_subject_bootstrap(tenant_id, subject_type, status, updated_at DESC);
