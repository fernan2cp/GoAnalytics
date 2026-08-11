package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

// ErrSnapshotRevisionExpired identifica el 409 revision_expired de la fuente.
var ErrSnapshotRevisionExpired = errors.New("revisión de snapshot vencida")

// BootstrapSubjects importa un snapshot paginado con checkpoint durable.
type BootstrapSubjects struct {
	source   outbound.SubjectSnapshotSource
	repo     outbound.SubjectBootstrapRepository
	metrics  outbound.Metrics
	pageSize int
}

// NewBootstrapSubjects construye el caso de uso de bootstrap.
func NewBootstrapSubjects(source outbound.SubjectSnapshotSource, repo outbound.SubjectBootstrapRepository, metrics outbound.Metrics, pageSize int) (*BootstrapSubjects, error) {
	if source == nil || repo == nil || pageSize < 1 || pageSize > 500 {
		return nil, fmt.Errorf("dependencias o tamaño de página de bootstrap inválidos")
	}
	return &BootstrapSubjects{source: source, repo: repo, metrics: metrics, pageSize: pageSize}, nil
}

// Run reanuda el último checkpoint y reinicia si la revisión dejó de existir.
func (useCase *BootstrapSubjects) Run(ctx context.Context) error {
	started := time.Now()
	state, err := useCase.repo.BeginOrResume(ctx)
	if err != nil {
		return err
	}
	restarts := 0
	for {
		page, fetchErr := useCase.source.FetchPage(ctx, state.Cursor, state.SourceRevision, useCase.pageSize)
		if errors.Is(fetchErr, ErrSnapshotRevisionExpired) {
			if restarts >= 2 {
				_ = useCase.repo.Fail(ctx, state, fetchErr.Error())
				return fetchErr
			}
			state, err = useCase.repo.RestartExpired(ctx, state)
			if err != nil {
				return err
			}
			restarts++
			continue
		}
		if fetchErr != nil {
			_ = useCase.repo.Fail(ctx, state, fetchErr.Error())
			return fetchErr
		}
		state, err = useCase.repo.ApplyPage(ctx, state, page)
		if err != nil {
			_ = useCase.repo.Fail(ctx, state, err.Error())
			if useCase.metrics != nil {
				useCase.metrics.IncrementCounter(outbound.MetricSubjectBootstrapPages, map[string]string{"status": "error"})
			}
			return err
		}
		if useCase.metrics != nil {
			useCase.metrics.IncrementCounter(outbound.MetricSubjectBootstrapPages, map[string]string{"status": "processed"})
			for range page.Rows {
				useCase.metrics.IncrementCounter(outbound.MetricSubjectBootstrapSubjects, map[string]string{"status": "processed"})
			}
		}
		if !page.HasMore {
			if useCase.metrics != nil {
				useCase.metrics.ObserveHistogram(outbound.MetricSubjectBootstrapDuration, time.Since(started).Seconds(), nil)
			}
			return nil
		}
	}
}
