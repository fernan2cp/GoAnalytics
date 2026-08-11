package outbound

import "context"

// ContextResolutionRepository procesa el histórico contextual por páginas.
type ContextResolutionRepository interface {
	LoadContextResolutionCheckpoint(ctx context.Context, revisionID int64) (int64, error)
	ResolveContextBatch(ctx context.Context, revisionID int64, afterID int64, limit int) (processed int64, lastID int64, err error)
}
