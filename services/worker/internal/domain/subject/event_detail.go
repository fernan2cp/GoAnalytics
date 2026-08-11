package subject

import "time"

const (
	// NormalizationNormalized indica una contribución apta para proyección.
	NormalizationNormalized = "normalized"
	// NormalizationInvalidSubject indica una forma de sujeto incompleta.
	NormalizationInvalidSubject = "invalid_subject"
	// NormalizationUnsupportedType indica un tipo fuera del MVP.
	NormalizationUnsupportedType = "unsupported_subject_type"
	// NormalizationTimezoneUnresolved indica que no pudo calcularse fecha local.
	NormalizationTimezoneUnresolved = "timezone_unresolved"
	// NormalizationIdempotencyConflict indica una contribución tenant duplicada.
	NormalizationIdempotencyConflict = "idempotency_conflict"
	// NormalizationLifecycleIgnored indica una mutación antigua auditable.
	NormalizationLifecycleIgnored = "lifecycle_ignored"

	// ContextRegistryDisabled indica que todavía no se consultó el registro.
	ContextRegistryDisabled = "registry_disabled"
	// ContextRegistered indica una definición activa y resuelta.
	ContextRegistered = "registered"
	// ContextUnregistered indica una identidad ausente del registro.
	ContextUnregistered = "unregistered"
	// ContextInactive indica una definición publicada pero inactiva.
	ContextInactive = "inactive_context"
	// ContextFamilyUnresolved indica una declaración contextual incompleta.
	ContextFamilyUnresolved = "family_unresolved"
)

// EventDetail representa un sujeto normalizado dentro de un evento crudo.
//
// SubjectOrdinal conserva la primera posición del payload. LocalDate y
// TimezoneName son el snapshot calendario aplicado; los IDs contextuales son
// opcionales hasta habilitar el registro.
type EventDetail struct {
	SubjectOrdinal              int
	SubjectType                 string
	ExternalSubjectID           string
	Relation                    string
	OperationType               string
	OperationID                 string
	ContextDefinitionID         *int64
	ContextKey                  string
	ContextVersion              *int
	ClaimedContextFamily        string
	ResolvedContextFamily       string
	ContextResolutionStatus     string
	ContextRegistryRevisionID   *int64
	TimezoneName                string
	LocalDate                   *time.Time
	NormalizationStatus         string
	NormalizationReason         string
	Metadata                    map[string]any
	InitialLifecycleStatus      LifecycleStatus
	LifecycleSourceUpdatedAt    *time.Time
	LifecycleSourceMutationID   string
	LifecycleEffectiveAt        *time.Time
	MergedIntoExternalSubjectID string
}
