package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/adapters/inbound/httphealth"
	"goanalytics/services/worker/internal/adapters/outbound/httpsnapshot"
	metricsadapter "goanalytics/services/worker/internal/adapters/outbound/metrics"
	postgresadapter "goanalytics/services/worker/internal/adapters/outbound/postgres"
	"goanalytics/services/worker/internal/application/usecases"
)

// SubjectJobsContainer agrupa recursos exclusivos del binario de jobs.
type SubjectJobsContainer struct {
	Postgres      *pgxpool.Pool
	Bootstrap     *usecases.BootstrapSubjects
	ProjectGlobal *usecases.ProjectGlobal
	Scheduler     *usecases.ProjectionScheduler
	Health        http.Handler
}

// NewSubjectJobsContainer construye el cliente, repositorio y caso de uso.
func NewSubjectJobsContainer(ctx context.Context, config Config) (*SubjectJobsContainer, error) {
	if !config.SubjectBootstrapEnabled && !config.SubjectProjectionEnabled {
		return nil, fmt.Errorf("jobs de sujetos deshabilitados")
	}
	pool, err := pgxpool.New(ctx, config.PostgresDSN())
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	metrics := metricsadapter.NewRegistry()
	container := &SubjectJobsContainer{Postgres: pool}
	if config.SubjectBootstrapEnabled {
		source, sourceErr := httpsnapshot.NewClient(config.SubjectSnapshotURL, config.SubjectSourceToken, config.SubjectSnapshotTimeout)
		if sourceErr != nil {
			pool.Close()
			return nil, sourceErr
		}
		repository, repositoryErr := postgresadapter.NewSubjectBootstrapRepository(pool)
		if repositoryErr != nil {
			pool.Close()
			return nil, repositoryErr
		}
		container.Bootstrap, err = usecases.NewBootstrapSubjects(source, repository, metrics, config.SubjectBootstrapPageSize)
		if err != nil {
			pool.Close()
			return nil, err
		}
	}
	if config.SubjectProjectionEnabled {
		repository, repositoryErr := postgresadapter.NewProjectionRepository(pool)
		if repositoryErr != nil {
			pool.Close()
			return nil, repositoryErr
		}
		container.ProjectGlobal, err = usecases.NewProjectGlobal(repository, metrics, config.SubjectProjectionVersion, config.SubjectProjectionBatchSize)
		if err != nil {
			pool.Close()
			return nil, err
		}
		container.Scheduler = usecases.NewProjectionScheduler(repository, container.ProjectGlobal, config.SubjectProjectionVersion, config.SubjectProjectionInterval, config.SubjectJobsCheckInterval)
	}
	ready := httphealth.NewReadyHandler(pool.Ping)
	operational := httphealth.NewRouterWithMetrics(httphealth.NewHealthHandler(), ready, metrics.Handler())
	mux := http.NewServeMux()
	mux.Handle("/", operational)
	mux.HandleFunc("/status", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		status := usecases.SchedulerStatus{}
		if container.Scheduler != nil {
			status = container.Scheduler.Status()
		}
		_ = json.NewEncoder(writer).Encode(status)
	})
	container.Health = mux
	return container, nil
}

// Close libera el pool PostgreSQL del proceso de jobs.
func (container *SubjectJobsContainer) Close() {
	if container != nil && container.Postgres != nil {
		container.Postgres.Close()
	}
}
