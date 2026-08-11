package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/domain/subject"
)

// ProjectionRepository persiste runs, checkpoints y agregados globales.
type ProjectionRepository struct{ pool *pgxpool.Pool }

// NewProjectionRepository crea el repositorio de proyección.
func NewProjectionRepository(pool *pgxpool.Pool) (*ProjectionRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool postgres requerido")
	}
	return &ProjectionRepository{pool: pool}, nil
}

// StartRun registra una ejecución running.
func (repository *ProjectionRepository) StartRun(ctx context.Context, name string, version int, from subject.ProjectionWatermark) (subject.ProjectionRun, error) {
	var run subject.ProjectionRun
	err := repository.pool.QueryRow(ctx, `INSERT INTO analytics_projection_runs(projection_name,projection_version,status,started_at,source_from_updated_at,source_from_id) VALUES($1,$2,'running',now(),$3,$4) RETURNING id,projection_name,projection_version,status,started_at`, name, version, nullableWatermarkTime(from), from.ID).Scan(&run.ID, &run.Name, &run.Version, &run.Status, &run.StartedAt)
	return run, err
}

// CompleteRun confirma métricas y watermark final del run.
func (repository *ProjectionRepository) CompleteRun(ctx context.Context, runID int64, to subject.ProjectionWatermark, affectedDays int, rowsRead, rowsWritten int64) error {
	_, err := repository.pool.Exec(ctx, `UPDATE analytics_projection_runs SET status='completed',completed_at=now(),source_to_updated_at=$2,source_to_id=$3,affected_days=$4,rows_read=$5,rows_written=$6 WHERE id=$1 AND status='running'`, runID, to.UpdatedAt, to.ID, affectedDays, rowsRead, rowsWritten)
	return err
}

// FailRun conserva un error sanitizado sin alterar agregados previos.
func (repository *ProjectionRepository) FailRun(ctx context.Context, runID int64, cause string) error {
	_, err := repository.pool.Exec(ctx, `UPDATE analytics_projection_runs SET status='failed',completed_at=now(),last_error=left($2,1000) WHERE id=$1 AND status='running'`, runID, cause)
	return err
}

// LoadCheckpoint devuelve cero cuando todavía no existe proyección.
func (repository *ProjectionRepository) LoadCheckpoint(ctx context.Context, name string, version int) (subject.ProjectionWatermark, error) {
	var watermark subject.ProjectionWatermark
	err := repository.pool.QueryRow(ctx, `SELECT last_updated_at,last_id FROM analytics_projection_checkpoints WHERE projection_name=$1 AND projection_version=$2`, name, version).Scan(&watermark.UpdatedAt, &watermark.ID)
	if err == pgx.ErrNoRows {
		return subject.ProjectionWatermark{}, nil
	}
	return watermark, err
}

// ReplaceDailyAndCheckpoint reemplaza días absolutos y avanza el checkpoint atómicamente.
func (repository *ProjectionRepository) ReplaceDailyAndCheckpoint(ctx context.Context, name string, version int, aggregates []subject.DailyAggregate, to subject.ProjectionWatermark) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, aggregate := range aggregates {
		_, err = tx.Exec(ctx, `INSERT INTO analytics_subject_tenant_daily(tenant_id,analytics_subject_id,local_date,timezone_name,impression_count,view_count,selection_count,association_count,last_association_at,projection_version,computed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now()) ON CONFLICT(tenant_id,analytics_subject_id,local_date,timezone_name,projection_version) DO UPDATE SET impression_count=EXCLUDED.impression_count,view_count=EXCLUDED.view_count,selection_count=EXCLUDED.selection_count,association_count=EXCLUDED.association_count,last_association_at=EXCLUDED.last_association_at,computed_at=EXCLUDED.computed_at,updated_at=now()`, aggregate.TenantID, aggregate.SubjectID, aggregate.LocalDate, aggregate.TimezoneName, aggregate.ImpressionCount, aggregate.ViewCount, aggregate.SelectionCount, aggregate.AssociationCount, aggregate.LastAssociationAt, aggregate.ProjectionVersion)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO analytics_projection_checkpoints(projection_name,projection_version,last_updated_at,last_id,updated_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT(projection_name,projection_version) DO UPDATE SET last_updated_at=EXCLUDED.last_updated_at,last_id=EXCLUDED.last_id,updated_at=now()`, name, version, to.UpdatedAt, to.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullableWatermarkTime(watermark subject.ProjectionWatermark) any {
	if watermark.UpdatedAt.IsZero() {
		return nil
	}
	return watermark.UpdatedAt
}
