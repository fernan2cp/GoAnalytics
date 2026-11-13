package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/subject"
)

// SubjectSnapshotSource obtiene páginas revisionadas desde el proyecto principal.
type SubjectSnapshotSource interface {
	FetchPage(ctx context.Context, cursor, sourceRevision string, limit int) (subject.SnapshotPage, error)
}

// SubjectBootstrapRepository persiste progreso e identidades de forma atómica.
type SubjectBootstrapRepository interface {
	TryBootstrapLock(ctx context.Context) (ProjectionLock, bool, error)
	BeginOrResume(ctx context.Context, forceNew ...bool) (subject.BootstrapState, error)
	ApplyPage(ctx context.Context, state subject.BootstrapState, page subject.SnapshotPage) (subject.BootstrapState, error)
	Fail(ctx context.Context, state subject.BootstrapState, cause string) error
}
