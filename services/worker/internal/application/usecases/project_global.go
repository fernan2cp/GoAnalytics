package usecases

import (
	"context"
	"fmt"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

const globalProjectionName = "subject_global"

// ProjectGlobal recalcula agregados afectados y avanza watermark por lote.
type ProjectGlobal struct {
	repository outbound.GlobalProjectionRepository
	metrics    outbound.Metrics
	version    int
	batchSize  int
}

// NewProjectGlobal construye el proyector global incremental.
func NewProjectGlobal(repository outbound.GlobalProjectionRepository, metrics outbound.Metrics, version, batchSize int) (*ProjectGlobal, error) {
	if repository == nil || version < 1 || batchSize < 1 {
		return nil, fmt.Errorf("configuración de proyección inválida")
	}
	return &ProjectGlobal{repository: repository, metrics: metrics, version: version, batchSize: batchSize}, nil
}

// Run ejecuta una sola proyección protegida por advisory lock.
func (useCase *ProjectGlobal) Run(ctx context.Context) error {
	lock, acquired, err := useCase.repository.TryLock(ctx, globalProjectionName, useCase.version)
	if err != nil {
		return err
	}
	if !acquired {
		if useCase.metrics != nil {
			useCase.metrics.IncrementCounter("subject_projection_runs_total", map[string]string{"projection": "global", "status": "skipped_locked"})
		}
		return useCase.repository.RecordSkippedLocked(ctx, globalProjectionName, useCase.version)
	}
	defer lock.Release(context.Background())
	checkpoint, err := useCase.repository.LoadCheckpoint(ctx, globalProjectionName, useCase.version)
	if err != nil {
		return err
	}
	run, err := useCase.repository.StartRun(ctx, globalProjectionName, useCase.version, checkpoint)
	if err != nil {
		return err
	}
	var rowsRead, rowsWritten int64
	affected := map[string]struct{}{}
	for {
		days, to, read, loadErr := useCase.repository.LoadAffectedDays(ctx, checkpoint, useCase.batchSize)
		if loadErr != nil {
			_ = useCase.repository.FailRun(ctx, run.ID, loadErr.Error())
			return loadErr
		}
		if read == 0 {
			break
		}
		written, recomputeErr := useCase.repository.RecomputeDaysAndCheckpoint(ctx, globalProjectionName, useCase.version, days, to)
		if recomputeErr != nil {
			_ = useCase.repository.FailRun(ctx, run.ID, recomputeErr.Error())
			return recomputeErr
		}
		rowsRead += read
		rowsWritten += written
		checkpoint = to
		for _, day := range days {
			affected[fmt.Sprintf("%s\x00%d\x00%s\x00%s", day.TenantID, day.SubjectID, day.LocalDate.Format("2006-01-02"), day.TimezoneName)] = struct{}{}
		}
		if read < int64(useCase.batchSize) {
			break
		}
	}
	if err = useCase.repository.CompleteRun(ctx, run.ID, checkpoint, len(affected), rowsRead, rowsWritten); err != nil {
		return err
	}
	if useCase.metrics != nil {
		useCase.metrics.IncrementCounter("subject_projection_runs_total", map[string]string{"projection": "global", "status": "completed"})
	}
	return nil
}
