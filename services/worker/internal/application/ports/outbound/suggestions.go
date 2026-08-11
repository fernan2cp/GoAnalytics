package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/subject"
)

// SuggestionRepository consulta exclusivamente agregados tenant-first.
type SuggestionRepository interface {
	ProjectionAvailability(ctx context.Context, version int) (subject.ProjectionAvailability, error)
	GlobalSuggestions(ctx context.Context, tenantID string, version, windowDays, limit int) ([]subject.Suggestion, error)
}
