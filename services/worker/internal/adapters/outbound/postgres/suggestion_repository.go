package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/domain/subject"
)

// SuggestionRepository consulta rankings preagregados con aislamiento tenant-first.
type SuggestionRepository struct{ pool *pgxpool.Pool }

// NewSuggestionRepository crea el repositorio de lectura.
func NewSuggestionRepository(pool *pgxpool.Pool) (*SuggestionRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool postgres requerido")
	}
	return &SuggestionRepository{pool: pool}, nil
}

// ProjectionAvailability obtiene el último run global completado.
func (repository *SuggestionRepository) ProjectionAvailability(ctx context.Context, version int, names ...string) (subject.ProjectionAvailability, error) {
	var availability subject.ProjectionAvailability
	name := "subject_global"
	if len(names) > 0 {
		name = names[0]
	}
	err := repository.pool.QueryRow(ctx, `SELECT projection_version,completed_at FROM analytics_projection_runs WHERE projection_name=$2 AND projection_version=$1 AND status='completed' ORDER BY completed_at DESC,id DESC LIMIT 1`, version, name).Scan(&availability.ProjectionVersion, &availability.ComputedAt)
	if err == pgx.ErrNoRows {
		return availability, nil
	}
	availability.Available = err == nil
	return availability, err
}

// GlobalSuggestions suma 90 días por sujeto y aplica orden determinista.
func (repository *SuggestionRepository) GlobalSuggestions(ctx context.Context, tenantID string, version, windowDays, limit int) ([]subject.Suggestion, error) {
	rows, err := repository.pool.Query(ctx, `SELECT identity.external_subject_id,sum(daily.association_count)::bigint,max(daily.last_association_at) FROM analytics_subject_tenant_daily daily JOIN analytics_subjects identity ON identity.id=daily.analytics_subject_id AND identity.tenant_id=daily.tenant_id WHERE daily.tenant_id=$1 AND daily.projection_version=$2 AND identity.lifecycle_status='active' AND daily.local_date >= ((now() AT TIME ZONE daily.timezone_name)::date - ($3::int - 1)) GROUP BY identity.external_subject_id ORDER BY sum(daily.association_count) DESC,max(daily.last_association_at) DESC NULLS LAST,identity.external_subject_id ASC LIMIT $4`, tenantID, version, windowDays, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]subject.Suggestion, 0)
	for rows.Next() {
		var item subject.Suggestion
		item.SourceLevel = "global"
		if err = rows.Scan(&item.ExternalSubjectID, &item.AssociationCount, &item.LastAssociationAt); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// ContextResolutionStatus distingue contexto desconocido de uno inactivo en la
// revisión vigente. Recibe identidad publicada y devuelve estado o error SQL.
func (repository *SuggestionRepository) ContextResolutionStatus(ctx context.Context, key string, version int) (string, error) {
	var status string
	err := repository.pool.QueryRow(ctx, `SELECT e.status FROM analytics_context_registry_revisions r JOIN analytics_context_registry_entries e ON e.registry_revision_id=r.id JOIN analytics_context_definitions d ON d.id=e.context_definition_id WHERE r.validation_status='valid' AND r.activated_at IS NOT NULL AND d.context_key=$1 AND d.context_version=$2`, key, version).Scan(&status)
	if err == pgx.ErrNoRows {
		return "unregistered", nil
	}
	if err != nil {
		return "", err
	}
	if status == "inactive" {
		return "inactive_context", nil
	}
	return "registered", nil
}

// ContextSuggestions devuelve ranking exacto si la definición está activa.
func (repository *SuggestionRepository) ContextSuggestions(ctx context.Context, tenantID, contextKey string, contextVersion, projectionVersion, windowDays, limit int) ([]subject.Suggestion, bool, error) {
	var definitionID int64
	err := repository.pool.QueryRow(ctx, `SELECT d.id FROM analytics_context_registry_revisions r JOIN analytics_context_registry_entries e ON e.registry_revision_id=r.id AND e.status='active' JOIN analytics_context_definitions d ON d.id=e.context_definition_id WHERE r.validation_status='valid' AND r.activated_at IS NOT NULL AND d.context_key=$1 AND d.context_version=$2 ORDER BY r.activated_at DESC LIMIT 1`, contextKey, contextVersion).Scan(&definitionID)
	if err == pgx.ErrNoRows {
		return []subject.Suggestion{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := repository.pool.Query(ctx, `SELECT identity.external_subject_id,sum(daily.association_count)::bigint,max(daily.last_association_at) FROM analytics_subject_context_daily daily JOIN analytics_subjects identity ON identity.id=daily.analytics_subject_id AND identity.tenant_id=daily.tenant_id WHERE daily.tenant_id=$1 AND daily.context_definition_id=$2 AND daily.projection_version=$3 AND identity.lifecycle_status='active' AND daily.local_date>=((now() AT TIME ZONE daily.timezone_name)::date-($4::int-1)) GROUP BY identity.external_subject_id ORDER BY sum(daily.association_count) DESC,max(daily.last_association_at) DESC NULLS LAST,identity.external_subject_id ASC LIMIT $5`, tenantID, definitionID, projectionVersion, windowDays, limit)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()
	results := make([]subject.Suggestion, 0)
	for rows.Next() {
		var item subject.Suggestion
		item.SourceLevel = "context"
		if err = rows.Scan(&item.ExternalSubjectID, &item.AssociationCount, &item.LastAssociationAt); err != nil {
			return nil, true, err
		}
		results = append(results, item)
	}
	return results, true, rows.Err()
}
