package postgres

import (
	"context"
	"testing"
	"time"

	"goanalytics/services/worker/internal/domain/subject"
)

func TestProjectionRepositoryIntegrationPersistsRunCheckpointAndAggregate(t *testing.T) {
	_, pool := newIntegrationEventRepository(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, projectionIntegrationSchemaSQL); err != nil {
		t.Fatal(err)
	}
	var subjectID int64
	if err := pool.QueryRow(ctx, `INSERT INTO analytics_subjects(tenant_id,subject_type,external_subject_id,lifecycle_status,status_updated_at) VALUES('tenant-1','customer','customer-1','active',now()) RETURNING id`).Scan(&subjectID); err != nil {
		t.Fatal(err)
	}
	repository, err := NewProjectionRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	run, err := repository.StartRun(ctx, "subject_global", 1, subject.ProjectionWatermark{})
	if err != nil {
		t.Fatal(err)
	}
	watermark := subject.ProjectionWatermark{UpdatedAt: time.Date(2026, 8, 10, 13, 0, 0, 0, time.UTC), ID: 44}
	last := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	aggregate := subject.DailyAggregate{TenantID: "tenant-1", SubjectID: subjectID, LocalDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), TimezoneName: "America/Argentina/Buenos_Aires", AssociationCount: 3, LastAssociationAt: &last, ProjectionVersion: 1}
	if err = repository.ReplaceDailyAndCheckpoint(ctx, "subject_global", 1, []subject.DailyAggregate{aggregate}, watermark); err != nil {
		t.Fatal(err)
	}
	if err = repository.CompleteRun(ctx, run.ID, watermark, 1, 4, 1); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.LoadCheckpoint(ctx, "subject_global", 1)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != 44 || !loaded.UpdatedAt.Equal(watermark.UpdatedAt) {
		t.Fatalf("watermark=%#v", loaded)
	}
	var count int64
	if err = pool.QueryRow(ctx, `SELECT association_count FROM analytics_subject_tenant_daily WHERE tenant_id='tenant-1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("association_count=%d", count)
	}
}

const projectionIntegrationSchemaSQL = `
CREATE TABLE analytics_projection_runs(id BIGSERIAL PRIMARY KEY,projection_name TEXT NOT NULL,projection_version INTEGER NOT NULL,status TEXT NOT NULL,started_at TIMESTAMPTZ NOT NULL,completed_at TIMESTAMPTZ,source_from_updated_at TIMESTAMPTZ,source_from_id BIGINT,source_to_updated_at TIMESTAMPTZ,source_to_id BIGINT,affected_days INTEGER NOT NULL DEFAULT 0,rows_read BIGINT NOT NULL DEFAULT 0,rows_written BIGINT NOT NULL DEFAULT 0,last_error TEXT,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX ux_projection_runs_running_test ON analytics_projection_runs(projection_name,projection_version) WHERE status='running';
CREATE TABLE analytics_projection_checkpoints(projection_name TEXT NOT NULL,projection_version INTEGER NOT NULL,last_updated_at TIMESTAMPTZ NOT NULL,last_id BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL,PRIMARY KEY(projection_name,projection_version));
CREATE TABLE analytics_subject_tenant_daily(tenant_id TEXT NOT NULL,analytics_subject_id BIGINT NOT NULL REFERENCES analytics_subjects(id) ON DELETE CASCADE,local_date DATE NOT NULL,timezone_name TEXT NOT NULL,impression_count BIGINT NOT NULL DEFAULT 0,view_count BIGINT NOT NULL DEFAULT 0,selection_count BIGINT NOT NULL DEFAULT 0,association_count BIGINT NOT NULL DEFAULT 0,last_association_at TIMESTAMPTZ,projection_version INTEGER NOT NULL,computed_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(tenant_id,analytics_subject_id,local_date,timezone_name,projection_version));
`
