package usecases

import (
	"context"
	"fmt"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

const contextProjectionName = "subject_context"

// ProjectContext recalcula agregados contextuales afectados.
type ProjectContext struct {
	repository         outbound.ContextProjectionRepository
	version, batchSize int
}

// NewProjectContext construye la proyección contextual incremental.
func NewProjectContext(repository outbound.ContextProjectionRepository, version, batchSize int) (*ProjectContext, error) {
	if repository == nil || version < 1 || batchSize < 1 {
		return nil, fmt.Errorf("configuración contextual inválida")
	}
	return &ProjectContext{repository: repository, version: version, batchSize: batchSize}, nil
}

// Run ejecuta una proyección protegida por lock independiente.
func (useCase *ProjectContext) Run(ctx context.Context) error {
	lock, acquired, err := useCase.repository.TryLock(ctx, contextProjectionName, useCase.version)
	if err != nil {
		return err
	}
	if !acquired {
		return useCase.repository.RecordSkippedLocked(ctx, contextProjectionName, useCase.version)
	}
	defer lock.Release(context.Background())
	checkpoint, err := useCase.repository.LoadCheckpoint(ctx, contextProjectionName, useCase.version)
	if err != nil {
		return err
	}
	run, err := useCase.repository.StartRun(ctx, contextProjectionName, useCase.version, checkpoint)
	if err != nil {
		return err
	}
	var readTotal, writtenTotal int64
	affected := map[string]struct{}{}
	for {
		days, to, read, loadErr := useCase.repository.LoadAffectedContextDays(ctx, checkpoint, useCase.batchSize)
		if loadErr != nil {
			_ = useCase.repository.FailRun(ctx, run.ID, loadErr.Error())
			return loadErr
		}
		if read == 0 {
			break
		}
		written, recomputeErr := useCase.repository.RecomputeContextDaysAndCheckpoint(ctx, contextProjectionName, useCase.version, days, to)
		if recomputeErr != nil {
			_ = useCase.repository.FailRun(ctx, run.ID, recomputeErr.Error())
			return recomputeErr
		}
		readTotal += read
		writtenTotal += written
		checkpoint = to
		for _, day := range days {
			affected[fmt.Sprintf("%s:%d:%d:%s:%s", day.TenantID, day.SubjectID, day.ContextDefinitionID, day.LocalDate.Format("2006-01-02"), day.TimezoneName)] = struct{}{}
		}
		if read < int64(useCase.batchSize) {
			break
		}
	}
	return useCase.repository.CompleteRun(ctx, run.ID, checkpoint, len(affected), readTotal, writtenTotal)
}

// ProjectionSequence ejecuta proyecciones relacionadas en orden estable.
type ProjectionSequence struct{ runners []ProjectionRunner }

// NewProjectionSequence crea una secuencia sin concurrencia interna.
func NewProjectionSequence(runners ...ProjectionRunner) *ProjectionSequence {
	return &ProjectionSequence{runners: runners}
}

// Run ejecuta cada proyección y detiene ante el primer error.
func (sequence *ProjectionSequence) Run(ctx context.Context) error {
	for _, runner := range sequence.runners {
		if err := runner.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
