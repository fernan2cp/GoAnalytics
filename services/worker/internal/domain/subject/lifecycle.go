package subject

import "time"

// LifecycleOrder representa la tupla monotónica recibida desde la fuente.
type LifecycleOrder struct {
	SourceUpdatedAt  time.Time
	SourceMutationID string
}

// IsAfter indica si una tupla lifecycle debe reemplazar a otra.
//
// Compara primero SourceUpdatedAt y, ante empate, SourceMutationID por bytes
// UTF-8. Devuelve false para una tupla igual o anterior. No usa effective_at.
func (incoming LifecycleOrder) IsAfter(current LifecycleOrder) bool {
	if incoming.SourceUpdatedAt.After(current.SourceUpdatedAt) {
		return true
	}
	if incoming.SourceUpdatedAt.Before(current.SourceUpdatedAt) {
		return false
	}
	return incoming.SourceMutationID > current.SourceMutationID
}
