package subject

import "time"

// Suggestion representa un resultado externo y agregado, sin claves internas.
type Suggestion struct {
	ExternalSubjectID string     `json:"external_subject_id"`
	AssociationCount  int64      `json:"association_count"`
	LastAssociationAt *time.Time `json:"last_association_at"`
	SourceLevel       string     `json:"source_level"`
}

// ProjectionAvailability describe versión, frescura y disponibilidad.
type ProjectionAvailability struct {
	Available         bool
	ProjectionVersion int
	ComputedAt        time.Time
}
