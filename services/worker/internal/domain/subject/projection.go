package subject

import "time"

// ProjectionWatermark define el orden total updated_at + id.
type ProjectionWatermark struct {
	UpdatedAt time.Time
	ID        int64
}

// DailyAggregate representa un reemplazo absoluto del agregado global.
type DailyAggregate struct {
	TenantID          string
	SubjectID         int64
	LocalDate         time.Time
	TimezoneName      string
	ImpressionCount   int64
	ViewCount         int64
	SelectionCount    int64
	AssociationCount  int64
	LastAssociationAt *time.Time
	ProjectionVersion int
}

// ProjectionRun conserva trazabilidad de una ejecución.
type ProjectionRun struct {
	ID          int64
	Name        string
	Version     int
	Status      string
	StartedAt   time.Time
	CompletedAt *time.Time
}

// AffectedDay identifica una clave diaria que debe recalcularse completa.
type AffectedDay struct {
	TenantID     string
	SubjectID    int64
	LocalDate    time.Time
	TimezoneName string
}
