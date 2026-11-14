CREATE TABLE IF NOT EXISTS analytics_projection_runs (
    id BIGSERIAL PRIMARY KEY,
    projection_name TEXT NOT NULL,
    projection_version INTEGER NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    source_from_updated_at TIMESTAMPTZ,
    source_from_id BIGINT,
    source_to_updated_at TIMESTAMPTZ,
    source_to_id BIGINT,
    affected_days INTEGER NOT NULL DEFAULT 0,
    rows_read BIGINT NOT NULL DEFAULT 0,
    rows_written BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_projection_runs_version CHECK (projection_version > 0),
    CONSTRAINT ck_projection_runs_status CHECK (status IN ('running','completed','failed','skipped_locked')),
    CONSTRAINT ck_projection_runs_counts CHECK (affected_days >= 0 AND rows_read >= 0 AND rows_written >= 0),
    CONSTRAINT ck_projection_runs_completion CHECK ((status = 'running') = (completed_at IS NULL))
);

CREATE INDEX IF NOT EXISTS ix_projection_runs_name_completed
ON analytics_projection_runs(projection_name, projection_version, completed_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_projection_runs_running
ON analytics_projection_runs(projection_name, projection_version) WHERE status = 'running';

CREATE TABLE IF NOT EXISTS analytics_projection_checkpoints (
    projection_name TEXT NOT NULL,
    projection_version INTEGER NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL,
    last_id BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (projection_name, projection_version),
    CONSTRAINT ck_projection_checkpoints_version CHECK (projection_version > 0),
    CONSTRAINT ck_projection_checkpoints_id CHECK (last_id >= 0)
);

CREATE TABLE IF NOT EXISTS analytics_subject_tenant_daily (
    tenant_id TEXT NOT NULL,
    analytics_subject_id BIGINT NOT NULL REFERENCES analytics_subjects(id) ON DELETE CASCADE,
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
    PRIMARY KEY (tenant_id, analytics_subject_id, local_date, timezone_name, projection_version),
    CONSTRAINT ck_subject_tenant_daily_version CHECK (projection_version > 0),
    CONSTRAINT ck_subject_tenant_daily_counts CHECK (
        impression_count >= 0 AND view_count >= 0 AND selection_count >= 0 AND association_count >= 0
    )
);

CREATE INDEX IF NOT EXISTS ix_subject_tenant_daily_ranking
ON analytics_subject_tenant_daily(tenant_id, local_date, association_count DESC, last_association_at DESC);

CREATE INDEX IF NOT EXISTS ix_subject_tenant_daily_window
ON analytics_subject_tenant_daily(tenant_id, projection_version, local_date DESC, analytics_subject_id);
