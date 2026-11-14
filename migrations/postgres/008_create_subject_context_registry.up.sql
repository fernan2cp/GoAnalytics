CREATE TABLE IF NOT EXISTS analytics_context_registry_revisions (
    id BIGSERIAL PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    registry_version BIGINT NOT NULL,
    registry_revision TEXT NOT NULL UNIQUE,
    payload JSONB NOT NULL,
    payload_checksum TEXT NOT NULL,
    validation_status TEXT NOT NULL,
    loaded_at TIMESTAMPTZ NOT NULL,
    activated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_context_registry_schema CHECK (schema_version = 1),
    CONSTRAINT ck_context_registry_version CHECK (registry_version > 0),
    CONSTRAINT ck_context_registry_validation CHECK (validation_status IN ('valid','invalid'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_context_registry_active
ON analytics_context_registry_revisions((validation_status)) WHERE validation_status='valid' AND activated_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS analytics_context_definitions (
    id BIGSERIAL PRIMARY KEY,
    context_key TEXT NOT NULL,
    context_version INTEGER NOT NULL,
    context_family TEXT NOT NULL,
    parent_definition_id BIGINT REFERENCES analytics_context_definitions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ux_context_definitions_identity UNIQUE(context_key,context_version),
    CONSTRAINT ck_context_definitions_version CHECK(context_version > 0),
    CONSTRAINT ck_context_definitions_family CHECK(context_family IN ('search','selection','transaction','service','support','content','navigation')),
    CONSTRAINT ck_context_definitions_parent CHECK(parent_definition_id IS NULL OR parent_definition_id <> id)
);

CREATE TABLE IF NOT EXISTS analytics_context_registry_entries (
    registry_revision_id BIGINT NOT NULL REFERENCES analytics_context_registry_revisions(id) ON DELETE CASCADE,
    context_definition_id BIGINT NOT NULL REFERENCES analytics_context_definitions(id),
    status TEXT NOT NULL,
    PRIMARY KEY(registry_revision_id,context_definition_id),
    CONSTRAINT ck_context_registry_entries_status CHECK(status IN ('active','inactive'))
);

CREATE TABLE IF NOT EXISTS analytics_unregistered_contexts (
    context_key TEXT NOT NULL,
    context_version INTEGER NOT NULL,
    claimed_context_family TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    event_count BIGINT NOT NULL,
    reconciliation_status TEXT NOT NULL,
    last_reconciliation_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    PRIMARY KEY(context_key,context_version),
    CONSTRAINT ck_unregistered_context_version CHECK(context_version > 0),
    CONSTRAINT ck_unregistered_context_count CHECK(event_count > 0),
    CONSTRAINT ck_unregistered_context_status CHECK(reconciliation_status IN ('pending','resolved','failed'))
);

CREATE TABLE IF NOT EXISTS analytics_subject_context_daily (
    tenant_id TEXT NOT NULL,
    analytics_subject_id BIGINT NOT NULL REFERENCES analytics_subjects(id) ON DELETE CASCADE,
    context_definition_id BIGINT NOT NULL REFERENCES analytics_context_definitions(id),
    local_date DATE NOT NULL,
    timezone_name TEXT NOT NULL,
    impression_count BIGINT NOT NULL DEFAULT 0,
    view_count BIGINT NOT NULL DEFAULT 0,
    selection_count BIGINT NOT NULL DEFAULT 0,
    association_count BIGINT NOT NULL DEFAULT 0,
    last_association_at TIMESTAMPTZ,
    projection_version INTEGER NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(tenant_id,analytics_subject_id,context_definition_id,local_date,timezone_name,projection_version),
    CONSTRAINT ck_subject_context_daily_counts CHECK(impression_count>=0 AND view_count>=0 AND selection_count>=0 AND association_count>=0),
    CONSTRAINT ck_subject_context_daily_version CHECK(projection_version>0)
);
CREATE INDEX IF NOT EXISTS ix_subject_context_daily_ranking
ON analytics_subject_context_daily(tenant_id,context_definition_id,local_date,association_count DESC,last_association_at DESC);
CREATE INDEX IF NOT EXISTS ix_subject_context_daily_window
ON analytics_subject_context_daily(tenant_id,context_definition_id,projection_version,local_date DESC,analytics_subject_id);

ALTER TABLE analytics_event_subjects
ADD CONSTRAINT fk_event_subjects_context_definition FOREIGN KEY(context_definition_id) REFERENCES analytics_context_definitions(id),
ADD CONSTRAINT fk_event_subjects_context_revision FOREIGN KEY(context_registry_revision_id) REFERENCES analytics_context_registry_revisions(id);
