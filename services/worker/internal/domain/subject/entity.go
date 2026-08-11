// Package subject contiene identidad e invariantes de los sujetos analíticos.
package subject

import "time"

// Type identifica la clase canónica de sujeto analítico.
type Type string

// CustomerType es el único tipo habilitado durante el MVP.
const CustomerType Type = "customer"

// LifecycleStatus representa la elegibilidad y estado de vida de un sujeto.
type LifecycleStatus string

const (
	// LifecycleUnknown identifica sujetos descubiertos sin confirmación del backend.
	LifecycleUnknown LifecycleStatus = "unknown"
	// LifecycleActive identifica sujetos confirmados y elegibles.
	LifecycleActive LifecycleStatus = "active"
	// LifecycleInactive identifica sujetos que dejaron de ser elegibles.
	LifecycleInactive LifecycleStatus = "inactive"
	// LifecycleMerged identifica sujetos fusionados con otro sujeto canónico.
	LifecycleMerged LifecycleStatus = "merged"
)

// Subject representa la identidad tenant-first y su estado de lifecycle.
//
// ExternalSubjectID es opaco y solo es canónico dentro de TenantID y Type.
// MergedIntoSubjectID referencia una clave interna únicamente cuando el estado
// es merged. Los campos SourceUpdatedAt y SourceMutationID forman juntos la
// marca de precedencia cuya aplicación se implementa en el caso de uso de
// lifecycle.
type Subject struct {
	ID                  int64
	TenantID            string
	Type                Type
	ExternalSubjectID   string
	LifecycleStatus     LifecycleStatus
	SourceUpdatedAt     *time.Time
	SourceMutationID    *string
	EffectiveAt         *time.Time
	MergedIntoSubjectID *int64
	StatusUpdatedAt     time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
