package usecases

import (
	"context"
	"sync"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

// ProjectionRunner ejecuta una proyección global completa.
type ProjectionRunner interface {
	Run(ctx context.Context) error
}

// SchedulerStatus expone estado operativo sin datos de tenant.
type SchedulerStatus struct {
	LastAttemptAt time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
}

// ProjectionScheduler evalúa vencimiento y ejecuta el proyector.
type ProjectionScheduler struct {
	repository              outbound.GlobalProjectionRepository
	runner                  ProjectionRunner
	version                 int
	interval, checkInterval time.Duration
	mu                      sync.RWMutex
	status                  SchedulerStatus
}

// NewProjectionScheduler crea el scheduler diario.
func NewProjectionScheduler(repository outbound.GlobalProjectionRepository, runner ProjectionRunner, version int, interval, checkInterval time.Duration) *ProjectionScheduler {
	return &ProjectionScheduler{repository: repository, runner: runner, version: version, interval: interval, checkInterval: checkInterval}
}

// RunIfDue ejecuta al iniciar si no existe éxito o si está vencido.
func (scheduler *ProjectionScheduler) RunIfDue(ctx context.Context, now time.Time) (bool, error) {
	completed, err := scheduler.repository.LastCompletedAt(ctx, globalProjectionName, scheduler.version)
	if err != nil {
		return false, err
	}
	if !completed.IsZero() && now.Sub(completed) < scheduler.interval {
		return false, nil
	}
	scheduler.mu.Lock()
	scheduler.status.LastAttemptAt = now
	scheduler.mu.Unlock()
	err = scheduler.runner.Run(ctx)
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	if err != nil {
		scheduler.status.LastError = err.Error()
		return true, err
	}
	scheduler.status.LastSuccessAt = now
	scheduler.status.LastError = ""
	return true, nil
}

// Serve evalúa al inicio y luego en intervalos cortos hasta cancelación.
func (scheduler *ProjectionScheduler) Serve(ctx context.Context) error {
	if _, err := scheduler.RunIfDue(ctx, time.Now().UTC()); err != nil {
		return err
	}
	ticker := time.NewTicker(scheduler.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-ticker.C:
			if _, err := scheduler.RunIfDue(ctx, now.UTC()); err != nil {
				return err
			}
		}
	}
}

// Status devuelve una copia segura del estado del scheduler.
func (scheduler *ProjectionScheduler) Status() SchedulerStatus {
	scheduler.mu.RLock()
	defer scheduler.mu.RUnlock()
	return scheduler.status
}
