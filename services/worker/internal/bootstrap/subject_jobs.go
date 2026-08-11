package bootstrap

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/adapters/outbound/httpsnapshot"
	metricsadapter "goanalytics/services/worker/internal/adapters/outbound/metrics"
	postgresadapter "goanalytics/services/worker/internal/adapters/outbound/postgres"
	"goanalytics/services/worker/internal/application/usecases"
)

// SubjectJobsContainer agrupa recursos exclusivos del binario de jobs.
type SubjectJobsContainer struct {
	Postgres  *pgxpool.Pool
	Bootstrap *usecases.BootstrapSubjects
}

// NewSubjectJobsContainer construye el cliente, repositorio y caso de uso.
func NewSubjectJobsContainer(ctx context.Context, config Config) (*SubjectJobsContainer, error) {
	if !config.SubjectBootstrapEnabled {
		return nil, fmt.Errorf("bootstrap de sujetos deshabilitado")
	}
	pool, err := pgxpool.New(ctx, config.PostgresDSN())
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	source, err := httpsnapshot.NewClient(config.SubjectSnapshotURL, config.SubjectSourceToken, config.SubjectSnapshotTimeout)
	if err != nil {
		pool.Close()
		return nil, err
	}
	repository, err := postgresadapter.NewSubjectBootstrapRepository(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	runner, err := usecases.NewBootstrapSubjects(source, repository, metricsadapter.NewRegistry(), config.SubjectBootstrapPageSize)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &SubjectJobsContainer{Postgres: pool, Bootstrap: runner}, nil
}

// Close libera el pool PostgreSQL del proceso de jobs.
func (container *SubjectJobsContainer) Close() {
	if container != nil && container.Postgres != nil {
		container.Postgres.Close()
	}
}
