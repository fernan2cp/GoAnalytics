package usecases

import (
	"context"
	"errors"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/subject"
)

var (
	// ErrProjectionUnavailable indica que nunca existe un run exitoso.
	ErrProjectionUnavailable = errors.New("projection_unavailable")
	// ErrProjectionStale indica que la política prohíbe servir datos vencidos.
	ErrProjectionStale = errors.New("projection_stale")
)

// SuggestionQuery contiene parámetros ya validados por el adaptador HTTP.
type SuggestionQuery struct {
	TenantID       string
	Limit          int
	ContextKey     string
	ContextVersion int
}

// SuggestionResponse conserva trazabilidad y resultados externos.
type SuggestionResponse struct {
	TenantID          string               `json:"tenant_id"`
	SubjectType       string               `json:"subject_type"`
	Context           map[string]any       `json:"context"`
	Window            map[string]int       `json:"window"`
	ComputedAt        time.Time            `json:"computed_at"`
	ProjectionVersion int                  `json:"projection_version"`
	IsStale           bool                 `json:"is_stale"`
	Results           []subject.Suggestion `json:"results"`
}

// QuerySuggestions aplica disponibilidad, frescura y ranking global.
type QuerySuggestions struct {
	repository                       outbound.SuggestionRepository
	version, globalDays, contextDays int
	staleAfter                       time.Duration
	serveStale                       bool
}

// NewQuerySuggestions construye el caso de uso de lectura.
func NewQuerySuggestions(repository outbound.SuggestionRepository, version, globalDays, contextDays int, staleAfter time.Duration, serveStale bool) *QuerySuggestions {
	return &QuerySuggestions{repository: repository, version: version, globalDays: globalDays, contextDays: contextDays, staleAfter: staleAfter, serveStale: serveStale}
}

// Execute devuelve global o errores estables de proyección.
func (useCase *QuerySuggestions) Execute(ctx context.Context, query SuggestionQuery, now time.Time) (SuggestionResponse, error) {
	availability, err := useCase.repository.ProjectionAvailability(ctx, useCase.version)
	if err != nil {
		return SuggestionResponse{}, err
	}
	if !availability.Available {
		return SuggestionResponse{}, ErrProjectionUnavailable
	}
	globalAvailability := availability
	contextResult := map[string]any{}
	var results []subject.Suggestion
	if query.ContextKey != "" {
		contextual, registered, contextErr := useCase.repository.ContextSuggestions(ctx, query.TenantID, query.ContextKey, query.ContextVersion, useCase.version, useCase.contextDays, query.Limit)
		if contextErr != nil {
			return SuggestionResponse{}, contextErr
		}
		status := "unregistered"
		if registered {
			status = "registered"
		} else {
			status, err = useCase.repository.ContextResolutionStatus(ctx, query.ContextKey, query.ContextVersion)
			if err != nil {
				return SuggestionResponse{}, err
			}
		}
		if len(contextual) > 0 {
			availability, err = useCase.repository.ProjectionAvailability(ctx, useCase.version, "subject_context")
			if err != nil {
				return SuggestionResponse{}, err
			}
			if !availability.Available {
				return SuggestionResponse{}, ErrProjectionUnavailable
			}
			results = contextual
			contextResult = map[string]any{"context_key": query.ContextKey, "context_version": query.ContextVersion, "resolution_status": status, "source_level": "context"}
			if len(results) < query.Limit {
				global, globalErr := useCase.repository.GlobalSuggestions(ctx, query.TenantID, useCase.version, useCase.globalDays, query.Limit)
				if globalErr != nil {
					return SuggestionResponse{}, globalErr
				}
				seen := make(map[string]bool, len(results))
				for _, item := range results {
					seen[item.ExternalSubjectID] = true
				}
				for _, item := range global {
					if len(results) >= query.Limit {
						break
					}
					if seen[item.ExternalSubjectID] {
						continue
					}
					seen[item.ExternalSubjectID] = true
					item.SourceLevel = "global"
					results = append(results, item)
					contextResult["source_level"] = "mixed"
					contextResult["fallback_applied"] = true
					if globalAvailability.ComputedAt.Before(availability.ComputedAt) {
						availability = globalAvailability
					}
				}
			}
		} else {
			contextResult = map[string]any{"context_key": query.ContextKey, "context_version": query.ContextVersion, "resolution_status": status, "source_level": "global", "fallback_applied": true}
		}
	}
	if results == nil {
		results, err = useCase.repository.GlobalSuggestions(ctx, query.TenantID, useCase.version, useCase.globalDays, query.Limit)
		if err != nil {
			return SuggestionResponse{}, err
		}
	}
	if results == nil {
		results = []subject.Suggestion{}
	}
	stale := now.Sub(availability.ComputedAt) > useCase.staleAfter
	if stale && !useCase.serveStale {
		return SuggestionResponse{}, ErrProjectionStale
	}
	return SuggestionResponse{TenantID: query.TenantID, SubjectType: "customer", Context: contextResult, Window: map[string]int{"context_days": useCase.contextDays, "global_days": useCase.globalDays}, ComputedAt: availability.ComputedAt, ProjectionVersion: availability.ProjectionVersion, IsStale: stale, Results: results}, nil
}
