package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"goanalytics/services/worker/internal/application/ports/outbound"

	"goanalytics/services/worker/internal/domain/subject"
)

// SubjectBootstrapRepository converge páginas y checkpoints en PostgreSQL.
type SubjectBootstrapRepository struct{ pool *pgxpool.Pool }

// NewSubjectBootstrapRepository crea el repositorio de bootstrap.
func NewSubjectBootstrapRepository(pool *pgxpool.Pool) (*SubjectBootstrapRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool postgres requerido")
	}
	return &SubjectBootstrapRepository{pool: pool}, nil
}

// BeginOrResume recupera la ejecución activa o crea una nueva.
// Recibe contexto y solicitud opcional de corte nuevo; exige el lock de bootstrap.
// Devuelve estado con checkpoint o errores de completado, expiración y PostgreSQL.
func (repository *SubjectBootstrapRepository) BeginOrResume(ctx context.Context, forceNew ...bool) (subject.BootstrapState, error) {
	var state subject.BootstrapState
	var status, cause string
	err := repository.pool.QueryRow(ctx, `SELECT id,COALESCE(cursor,''),COALESCE(source_revision,''),processed_count,failed_count,status,COALESCE(last_error,'') FROM analytics_subject_bootstrap WHERE subject_type='customer' ORDER BY id DESC LIMIT 1`).Scan(&state.ID, &state.Cursor, &state.SourceRevision, &state.ProcessedCount, &state.FailedCount, &status, &cause)
	if len(forceNew) > 0 && forceNew[0] {
		if err != nil && err != pgx.ErrNoRows {
			return state, err
		}
		return repository.RestartExpired(ctx, state)
	}
	if err == nil {
		if status == "completed" {
			return state, subject.ErrBootstrapCompleted
		}
		if cause == "snapshot_revision_expired" {
			return state, subject.ErrBootstrapExpired
		}
		_, err = repository.pool.Exec(ctx, `UPDATE analytics_subject_bootstrap SET status='running',completed_at=NULL,last_error=NULL,updated_at=now() WHERE id=$1`, state.ID)
		if err != nil {
			return state, err
		}
		return state, nil
	}
	if err != pgx.ErrNoRows {
		return state, err
	}
	err = repository.pool.QueryRow(ctx, `INSERT INTO analytics_subject_bootstrap(subject_type,status,started_at) VALUES('customer','running',now()) RETURNING id`).Scan(&state.ID)
	return state, err
}

// TryBootstrapLock excluye importaciones concurrentes durante todo el recorrido.
// Recibe contexto y devuelve lock de sesión, adquisición y error de PostgreSQL.
func (repository *SubjectBootstrapRepository) TryBootstrapLock(ctx context.Context) (outbound.ProjectionLock, bool, error) {
	projection := &ProjectionRepository{pool: repository.pool}
	return projection.TryLock(ctx, "subject_bootstrap", 1)
}

// ApplyPage aplica lifecycle monotónico y avanza el cursor en la misma transacción.
func (repository *SubjectBootstrapRepository) ApplyPage(ctx context.Context, state subject.BootstrapState, page subject.SnapshotPage) (subject.BootstrapState, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return state, err
	}
	defer tx.Rollback(ctx)
	for _, row := range page.Rows {
		if err = upsertSnapshotSubject(ctx, tx, row); err != nil {
			return state, err
		}
	}
	status := "running"
	if !page.HasMore {
		status = "completed"
	}
	err = tx.QueryRow(ctx, `UPDATE analytics_subject_bootstrap SET cursor=$2,source_revision=$3,processed_count=processed_count+$4,status=$5,completed_at=CASE WHEN $5='completed' THEN now() ELSE NULL END,updated_at=now() WHERE id=$1 RETURNING id,COALESCE(cursor,''),COALESCE(source_revision,''),processed_count,failed_count`, state.ID, page.NextCursor, page.SourceRevision, len(page.Rows), status).Scan(&state.ID, &state.Cursor, &state.SourceRevision, &state.ProcessedCount, &state.FailedCount)
	if err != nil {
		return state, err
	}
	return state, tx.Commit(ctx)
}

func upsertSnapshotSubject(ctx context.Context, tx pgx.Tx, row subject.SnapshotRow) error {
	var targetID any
	if row.LifecycleStatus == subject.LifecycleMerged {
		var id int64
		err := tx.QueryRow(ctx, `INSERT INTO analytics_subjects(tenant_id,subject_type,external_subject_id,lifecycle_status,status_updated_at) VALUES($1,'customer',$2,'unknown',now()) ON CONFLICT(tenant_id,subject_type,external_subject_id) DO UPDATE SET updated_at=analytics_subjects.updated_at RETURNING id`, row.TenantID, row.MergedIntoExternalSubjectID).Scan(&id)
		if err != nil {
			return err
		}
		targetID = id
	}
	_, err := tx.Exec(ctx, `INSERT INTO analytics_subjects(tenant_id,subject_type,external_subject_id,lifecycle_status,lifecycle_source_updated_at,lifecycle_source_mutation_id,merged_into_subject_id,status_updated_at) VALUES($1,'customer',$2,$3,$4,$5,$6,$4) ON CONFLICT(tenant_id,subject_type,external_subject_id) DO UPDATE SET lifecycle_status=EXCLUDED.lifecycle_status,lifecycle_source_updated_at=EXCLUDED.lifecycle_source_updated_at,lifecycle_source_mutation_id=EXCLUDED.lifecycle_source_mutation_id,merged_into_subject_id=EXCLUDED.merged_into_subject_id,status_updated_at=EXCLUDED.status_updated_at,updated_at=now() WHERE analytics_subjects.lifecycle_source_updated_at IS NULL OR EXCLUDED.lifecycle_source_updated_at>analytics_subjects.lifecycle_source_updated_at OR (EXCLUDED.lifecycle_source_updated_at=analytics_subjects.lifecycle_source_updated_at AND (EXCLUDED.lifecycle_source_mutation_id COLLATE "C")>(analytics_subjects.lifecycle_source_mutation_id COLLATE "C"))`, row.TenantID, row.ExternalSubjectID, row.LifecycleStatus, row.SourceUpdatedAt, row.SourceMutationID, targetID)
	return err
}

// RestartExpired abre un corte nuevo por solicitud explícita del operador.
func (repository *SubjectBootstrapRepository) RestartExpired(ctx context.Context, state subject.BootstrapState) (subject.BootstrapState, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return state, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE analytics_subject_bootstrap SET status='failed',completed_at=now(),last_error='replaced_by_operator',updated_at=now() WHERE id=$1 AND status='running'`, state.ID); err != nil {
		return state, err
	}
	var next subject.BootstrapState
	err = tx.QueryRow(ctx, `INSERT INTO analytics_subject_bootstrap(subject_type,status,started_at) VALUES('customer','running',now()) RETURNING id`).Scan(&next.ID)
	if err != nil {
		return next, err
	}
	return next, tx.Commit(ctx)
}

// Fail conserva el último checkpoint y registra el error operativo.
func (repository *SubjectBootstrapRepository) Fail(ctx context.Context, state subject.BootstrapState, cause string) error {
	_, err := repository.pool.Exec(ctx, `UPDATE analytics_subject_bootstrap SET status='failed',completed_at=now(),last_error=left($2,1000),updated_at=now() WHERE id=$1`, state.ID, cause)
	return err
}
