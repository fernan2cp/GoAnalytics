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
	stale := now.Sub(availability.ComputedAt) > useCase.staleAfter
	if stale && !useCase.serveStale {
		return SuggestionResponse{}, ErrProjectionStale
	}
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
		}
		if len(contextual) > 0 {
			results = contextual
			contextResult = map[string]any{"context_key": query.ContextKey, "context_version": query.ContextVersion, "resolution_status": status, "source_level": "context"}
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
	return SuggestionResponse{TenantID: query.TenantID, SubjectType: "customer", Context: contextResult, Window: map[string]int{"context_days": useCase.contextDays, "global_days": useCase.globalDays}, ComputedAt: availability.ComputedAt, ProjectionVersion: availability.ProjectionVersion, IsStale: stale, Results: results}, nil
}
