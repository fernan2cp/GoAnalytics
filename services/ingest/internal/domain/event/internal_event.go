package event

import "time"

const (
	// SourceWeb identifica eventos recibidos por el contrato público existente.
	SourceWeb = "web"
	// SourceServer identifica eventos autenticados por el contrato interno.
	SourceServer = "server"
)

// SubjectReference identifica un sujeto externo dentro de un evento interno.
//
// SubjectID es un identificador opaco y Relation describe su participación en
// el hecho. El MVP solo admite SubjectType customer.
type SubjectReference struct {
	SubjectType string
	SubjectID   string
	Relation    string
}

// OperationReference identifica opcionalmente el hecho de negocio asociado.
type OperationReference struct {
	OperationType string
	OperationID   string
}

// LifecycleChange contiene la precedencia y el destino de una mutación.
//
// SourceUpdatedAt y SourceMutationID forman la tupla monotónica. EffectiveAt
// es informativo y MergedIntoSubjectID solo se usa en subject_merged.
type LifecycleChange struct {
	SourceUpdatedAt     time.Time
	SourceMutationID    string
	EffectiveAt         *time.Time
	MergedIntoSubjectID string
}

// InternalEvent representa el contrato server-to-server ya decodificado.
type InternalEvent struct {
	EventID        string
	IdempotencyKey string
	EventName      string
	EventVersion   int
	Timestamp      time.Time
	SiteCode       string
	Subjects       []SubjectReference
	Operation      *OperationReference
	Lifecycle      *LifecycleChange
	Context        map[string]any
}
