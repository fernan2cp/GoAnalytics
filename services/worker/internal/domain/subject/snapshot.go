package subject

import "time"

// SnapshotRow representa una identidad técnica recibida desde la fuente.
type SnapshotRow struct {
	TenantID                    string
	ExternalSubjectID           string
	LifecycleStatus             LifecycleStatus
	SourceUpdatedAt             time.Time
	SourceMutationID            string
	MergedIntoExternalSubjectID string
}

// SnapshotPage conserva la revisión y el cursor opaco de una página.
type SnapshotPage struct {
	SourceRevision string
	NextCursor     string
	HasMore        bool
	Rows           []SnapshotRow
}

// BootstrapState describe una ejecución reanudable persistida.
type BootstrapState struct {
	ID             int64
	Cursor         string
	SourceRevision string
	ProcessedCount int64
	FailedCount    int64
}
