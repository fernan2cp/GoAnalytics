package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/subject"
)

// ProjectionRepository persiste runs, checkpoints y agregados globales.
type ProjectionRepository struct{ pool *pgxpool.Pool }

type projectionLock struct {
	connection *pgxpool.Conn
	key        string
}

func (lock *projectionLock) Release(ctx context.Context) error {
	if lock == nil || lock.connection == nil {
		return nil
	}
	defer lock.connection.Release()
	_, err := lock.connection.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, lock.key)
	return err
}

// TryLock intenta retener exclusión por nombre y versión en una conexión dedicada.
func (repository *ProjectionRepository) TryLock(ctx context.Context, name string, version int) (outbound.ProjectionLock, bool, error) {
	connection, err := repository.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	key := fmt.Sprintf("%s:%d", name, version)
	var acquired bool
	if err = connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&acquired); err != nil {
		connection.Release()
		return nil, false, err
	}
	if !acquired {
		connection.Release()
		return nil, false, nil
	}
	return &projectionLock{connection: connection, key: key}, true, nil
}

// RecordSkippedLocked registra una ejecución que no obtuvo exclusión.
func (repository *ProjectionRepository) RecordSkippedLocked(ctx context.Context, name string, version int) error {
	_, err := repository.pool.Exec(ctx, `INSERT INTO analytics_projection_runs(projection_name,projection_version,status,started_at,completed_at) VALUES($1,$2,'skipped_locked',now(),now())`, name, version)
	return err
}

// NewProjectionRepository crea el repositorio de proyección.
func NewProjectionRepository(pool *pgxpool.Pool) (*ProjectionRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool postgres requerido")
	}
	return &ProjectionRepository{pool: pool}, nil
}

// StartRun registra una ejecución running.
func (repository *ProjectionRepository) StartRun(ctx context.Context, name string, version int, from subject.ProjectionWatermark) (subject.ProjectionRun, error) {
	if _, err := repository.pool.Exec(ctx, `UPDATE analytics_projection_runs SET status='failed',completed_at=now(),last_error='recovered_after_interruption' WHERE projection_name=$1 AND projection_version=$2 AND status='running'`, name, version); err != nil {
		return subject.ProjectionRun{}, err
	}
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

// LastCompletedAt obtiene el último éxito para evaluar vencimiento.
func (repository *ProjectionRepository) LastCompletedAt(ctx context.Context, name string, version int) (time.Time, error) {
	var completed time.Time
	err := repository.pool.QueryRow(ctx, `SELECT completed_at FROM analytics_projection_runs WHERE projection_name=$1 AND projection_version=$2 AND status='completed' ORDER BY completed_at DESC,id DESC LIMIT 1`, name, version).Scan(&completed)
	if err == pgx.ErrNoRows {
		return time.Time{}, nil
	}
	return completed, err
}

// LoadAffectedDays lee el siguiente lote en orden total y deduplica sus claves.
func (repository *ProjectionRepository) LoadAffectedDays(ctx context.Context, after subject.ProjectionWatermark, limit int) ([]subject.AffectedDay, subject.ProjectionWatermark, int64, error) {
	fromTime := after.UpdatedAt
	if fromTime.IsZero() {
		fromTime = time.Unix(0, 0).UTC()
	}
	rows, err := repository.pool.Query(ctx, `SELECT tenant_id,analytics_subject_id,local_date,timezone_name,updated_at,id FROM analytics_event_subjects WHERE normalization_status='normalized' AND analytics_subject_id IS NOT NULL AND local_date IS NOT NULL AND timezone_name IS NOT NULL AND (updated_at>$1 OR (updated_at=$1 AND id>$2)) ORDER BY updated_at,id LIMIT $3`, fromTime, after.ID, limit)
	if err != nil {
		return nil, after, 0, err
	}
	defer rows.Close()
	keys := map[string]subject.AffectedDay{}
	watermark := after
	var read int64
	for rows.Next() {
		var day subject.AffectedDay
		if err = rows.Scan(&day.TenantID, &day.SubjectID, &day.LocalDate, &day.TimezoneName, &watermark.UpdatedAt, &watermark.ID); err != nil {
			return nil, after, read, err
		}
		key := fmt.Sprintf("%s\x00%d\x00%s\x00%s", day.TenantID, day.SubjectID, day.LocalDate.Format("2006-01-02"), day.TimezoneName)
		keys[key] = day
		read++
	}
	if err = rows.Err(); err != nil {
		return nil, after, read, err
	}
	days := make([]subject.AffectedDay, 0, len(keys))
	for _, day := range keys {
		days = append(days, day)
	}
	return days, watermark, read, nil
}

// RecomputeDaysAndCheckpoint reemplaza cada clave desde la fuente normalizada.
func (repository *ProjectionRepository) RecomputeDaysAndCheckpoint(ctx context.Context, name string, version int, days []subject.AffectedDay, to subject.ProjectionWatermark) (int64, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var written int64
	for _, day := range days {
		if _, err = tx.Exec(ctx, `DELETE FROM analytics_subject_tenant_daily WHERE tenant_id=$1 AND analytics_subject_id=$2 AND local_date=$3 AND timezone_name=$4 AND projection_version=$5`, day.TenantID, day.SubjectID, day.LocalDate, day.TimezoneName, version); err != nil {
			return written, err
		}
		result, insertErr := tx.Exec(ctx, `INSERT INTO analytics_subject_tenant_daily(tenant_id,analytics_subject_id,local_date,timezone_name,impression_count,view_count,selection_count,association_count,last_association_at,projection_version,computed_at) SELECT detail.tenant_id,detail.analytics_subject_id,detail.local_date,detail.timezone_name,count(*) FILTER(WHERE detail.event_name='subject_impression'),count(*) FILTER(WHERE detail.event_name='subject_viewed'),count(*) FILTER(WHERE detail.event_name='subject_selected'),count(*) FILTER(WHERE detail.event_name='subject_associated'),max(detail.event_time) FILTER(WHERE detail.event_name='subject_associated'),$5,now() FROM analytics_event_subjects detail JOIN analytics_subjects identity ON identity.id=detail.analytics_subject_id WHERE detail.tenant_id=$1 AND detail.analytics_subject_id=$2 AND detail.local_date=$3 AND detail.timezone_name=$4 AND detail.normalization_status='normalized' AND detail.event_name IN('subject_impression','subject_viewed','subject_selected','subject_associated') GROUP BY detail.tenant_id,detail.analytics_subject_id,detail.local_date,detail.timezone_name`, day.TenantID, day.SubjectID, day.LocalDate, day.TimezoneName, version)
		if insertErr != nil {
			return written, insertErr
		}
		written += result.RowsAffected()
	}
	_, err = tx.Exec(ctx, `INSERT INTO analytics_projection_checkpoints(projection_name,projection_version,last_updated_at,last_id,updated_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT(projection_name,projection_version) DO UPDATE SET last_updated_at=EXCLUDED.last_updated_at,last_id=EXCLUDED.last_id,updated_at=now()`, name, version, to.UpdatedAt, to.ID)
	if err != nil {
		return written, err
	}
	return written, tx.Commit(ctx)
}

// LoadAffectedContextDays lee contribuciones registradas por watermark.
func (repository *ProjectionRepository) LoadAffectedContextDays(ctx context.Context, after subject.ProjectionWatermark, limit int) ([]subject.AffectedContextDay, subject.ProjectionWatermark, int64, error) {
	fromTime := after.UpdatedAt
	if fromTime.IsZero() {
		fromTime = time.Unix(0, 0).UTC()
	}
	rows, err := repository.pool.Query(ctx, `SELECT tenant_id,analytics_subject_id,context_definition_id,local_date,timezone_name,updated_at,id FROM analytics_event_subjects WHERE normalization_status='normalized' AND context_resolution_status='registered' AND analytics_subject_id IS NOT NULL AND context_definition_id IS NOT NULL AND local_date IS NOT NULL AND timezone_name IS NOT NULL AND (updated_at>$1 OR(updated_at=$1 AND id>$2)) ORDER BY updated_at,id LIMIT $3`, fromTime, after.ID, limit)
	if err != nil {
		return nil, after, 0, err
	}
	defer rows.Close()
	keys := map[string]subject.AffectedContextDay{}
	watermark := after
	var read int64
	for rows.Next() {
		var day subject.AffectedContextDay
		if err = rows.Scan(&day.TenantID, &day.SubjectID, &day.ContextDefinitionID, &day.LocalDate, &day.TimezoneName, &watermark.UpdatedAt, &watermark.ID); err != nil {
			return nil, after, read, err
		}
		keys[fmt.Sprintf("%s\x00%d\x00%d\x00%s\x00%s", day.TenantID, day.SubjectID, day.ContextDefinitionID, day.LocalDate.Format("2006-01-02"), day.TimezoneName)] = day
		read++
	}
	if err = rows.Err(); err != nil {
		return nil, after, read, err
	}
	days := make([]subject.AffectedContextDay, 0, len(keys))
	for _, day := range keys {
		days = append(days, day)
	}
	return days, watermark, read, nil
}

// RecomputeContextDaysAndCheckpoint reemplaza días contextuales atómicamente.
func (repository *ProjectionRepository) RecomputeContextDaysAndCheckpoint(ctx context.Context, name string, version int, days []subject.AffectedContextDay, to subject.ProjectionWatermark) (int64, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var written int64
	for _, day := range days {
		if _, err = tx.Exec(ctx, `DELETE FROM analytics_subject_context_daily WHERE tenant_id=$1 AND analytics_subject_id=$2 AND context_definition_id=$3 AND local_date=$4 AND timezone_name=$5 AND projection_version=$6`, day.TenantID, day.SubjectID, day.ContextDefinitionID, day.LocalDate, day.TimezoneName, version); err != nil {
			return written, err
		}
		result, insertErr := tx.Exec(ctx, `INSERT INTO analytics_subject_context_daily(tenant_id,analytics_subject_id,context_definition_id,local_date,timezone_name,impression_count,view_count,selection_count,association_count,last_association_at,projection_version,computed_at) SELECT d.tenant_id,d.analytics_subject_id,d.context_definition_id,d.local_date,d.timezone_name,count(*) FILTER(WHERE d.event_name='subject_impression'),count(*) FILTER(WHERE d.event_name='subject_viewed'),count(*) FILTER(WHERE d.event_name='subject_selected'),count(*) FILTER(WHERE d.event_name='subject_associated'),max(d.event_time) FILTER(WHERE d.event_name='subject_associated'),$6,now() FROM analytics_event_subjects d JOIN analytics_subjects s ON s.id=d.analytics_subject_id WHERE d.tenant_id=$1 AND d.analytics_subject_id=$2 AND d.context_definition_id=$3 AND d.local_date=$4 AND d.timezone_name=$5 AND d.normalization_status='normalized' AND d.context_resolution_status='registered' AND d.event_name IN('subject_impression','subject_viewed','subject_selected','subject_associated') GROUP BY d.tenant_id,d.analytics_subject_id,d.context_definition_id,d.local_date,d.timezone_name`, day.TenantID, day.SubjectID, day.ContextDefinitionID, day.LocalDate, day.TimezoneName, version)
		if insertErr != nil {
			return written, insertErr
		}
		written += result.RowsAffected()
	}
	_, err = tx.Exec(ctx, `INSERT INTO analytics_projection_checkpoints(projection_name,projection_version,last_updated_at,last_id,updated_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT(projection_name,projection_version) DO UPDATE SET last_updated_at=EXCLUDED.last_updated_at,last_id=EXCLUDED.last_id,updated_at=now()`, name, version, to.UpdatedAt, to.ID)
	if err != nil {
		return written, err
	}
	return written, tx.Commit(ctx)
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
