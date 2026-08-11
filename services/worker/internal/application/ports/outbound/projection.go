package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/subject"
)

// ProjectionLock representa un advisory lock retenido en una conexión.
type ProjectionLock interface {
	Release(ctx context.Context) error
}

// GlobalProjectionRepository abstrae lectura incremental y publicación atómica.
type GlobalProjectionRepository interface {
	TryLock(ctx context.Context, name string, version int) (ProjectionLock, bool, error)
	RecordSkippedLocked(ctx context.Context, name string, version int) error
	StartRun(ctx context.Context, name string, version int, from subject.ProjectionWatermark) (subject.ProjectionRun, error)
	CompleteRun(ctx context.Context, runID int64, to subject.ProjectionWatermark, affectedDays int, rowsRead, rowsWritten int64) error
	FailRun(ctx context.Context, runID int64, cause string) error
	LoadCheckpoint(ctx context.Context, name string, version int) (subject.ProjectionWatermark, error)
	LoadAffectedDays(ctx context.Context, after subject.ProjectionWatermark, limit int) ([]subject.AffectedDay, subject.ProjectionWatermark, int64, error)
	RecomputeDaysAndCheckpoint(ctx context.Context, name string, version int, days []subject.AffectedDay, to subject.ProjectionWatermark) (int64, error)
}
