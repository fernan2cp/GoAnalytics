package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ContextResolutionRepository resuelve el histórico y avanza su checkpoint.
type ContextResolutionRepository struct{ pool *pgxpool.Pool }

// NewContextResolutionRepository crea el adaptador PostgreSQL.
func NewContextResolutionRepository(pool *pgxpool.Pool) (*ContextResolutionRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool requerido")
	}
	return &ContextResolutionRepository{pool: pool}, nil
}

// LoadContextResolutionCheckpoint devuelve el último ID confirmado.
func (repository *ContextResolutionRepository) LoadContextResolutionCheckpoint(ctx context.Context, revisionID int64) (int64, error) {
	var id int64
	err := repository.pool.QueryRow(ctx, `SELECT last_id FROM analytics_projection_checkpoints WHERE projection_name=$1 AND projection_version=1`, resolutionCheckpointName(revisionID)).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return id, err
}

// ResolveContextBatch clasifica una página y confirma el checkpoint en la misma transacción.
func (repository *ContextResolutionRepository) ResolveContextBatch(ctx context.Context, revisionID int64, afterID int64, limit int) (int64, int64, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return 0, afterID, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id FROM analytics_event_subjects WHERE id>$1 AND context_registry_revision_id IS NULL ORDER BY id LIMIT $2 FOR UPDATE SKIP LOCKED`, afterID, limit)
	if err != nil {
		return 0, afterID, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, afterID, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, afterID, err
	}
	if len(ids) == 0 {
		return 0, afterID, nil
	}
	lastID := ids[len(ids)-1]
	_, err = tx.Exec(ctx, `WITH candidates AS (SELECT d.id AS detail_id,def.id AS definition_id,def.context_family,e.status FROM analytics_event_subjects d LEFT JOIN analytics_context_definitions def ON def.context_key=d.context_key AND def.context_version=d.context_version LEFT JOIN analytics_context_registry_entries e ON e.context_definition_id=def.id AND e.registry_revision_id=$1 WHERE d.id=ANY($2)) UPDATE analytics_event_subjects d SET context_registry_revision_id=$1,context_definition_id=c.definition_id,resolved_context_family=c.context_family,context_resolution_status=CASE WHEN d.context_key IS NULL OR d.context_version IS NULL THEN 'family_unresolved' WHEN c.definition_id IS NULL OR c.status IS NULL THEN 'unregistered' WHEN c.status='inactive' THEN 'inactive_context' ELSE 'registered' END,updated_at=now() FROM candidates c WHERE d.id=c.detail_id`, revisionID, ids)
	if err != nil {
		return 0, afterID, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO analytics_unregistered_contexts(context_key,context_version,claimed_context_family,first_seen_at,last_seen_at,event_count,reconciliation_status) SELECT context_key,context_version,max(claimed_context_family),min(event_time),max(event_time),count(*),'pending' FROM analytics_event_subjects WHERE id=ANY($1) AND context_resolution_status='unregistered' GROUP BY context_key,context_version ON CONFLICT(context_key,context_version) DO UPDATE SET last_seen_at=GREATEST(analytics_unregistered_contexts.last_seen_at,EXCLUDED.last_seen_at),event_count=analytics_unregistered_contexts.event_count+EXCLUDED.event_count,reconciliation_status='pending'`, ids)
	if err != nil {
		return 0, afterID, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO analytics_projection_checkpoints(projection_name,projection_version,last_updated_at,last_id,updated_at) VALUES($1,1,now(),$2,now()) ON CONFLICT(projection_name,projection_version) DO UPDATE SET last_updated_at=now(),last_id=EXCLUDED.last_id,updated_at=now()`, resolutionCheckpointName(revisionID), lastID)
	if err != nil {
		return 0, afterID, err
	}
	return int64(len(ids)), lastID, tx.Commit(ctx)
}

func resolutionCheckpointName(revisionID int64) string {
	return fmt.Sprintf("subject_context_initial:%d", revisionID)
}
