package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/worker/internal/adapters/inbound/httphealth"
	"goanalytics/services/worker/internal/adapters/outbound/httpregistry"
	"goanalytics/services/worker/internal/adapters/outbound/httpsnapshot"
	metricsadapter "goanalytics/services/worker/internal/adapters/outbound/metrics"
	postgresadapter "goanalytics/services/worker/internal/adapters/outbound/postgres"
	redisadapter "goanalytics/services/worker/internal/adapters/outbound/redis"
	"goanalytics/services/worker/internal/application/usecases"
)

// SubjectJobsContainer agrupa recursos exclusivos del binario de jobs.
type SubjectJobsContainer struct {
	Postgres       *pgxpool.Pool
	Bootstrap      *usecases.BootstrapSubjects
	ProjectGlobal  *usecases.ProjectGlobal
	ProjectContext *usecases.ProjectContext
	Scheduler      *usecases.ProjectionScheduler
	Registry       *usecases.ContextRegistryLoader
	ResolveInitial *usecases.ResolveInitialContexts
	Redis          *goredis.Client
	Health         http.Handler
}

// NewSubjectJobsContainer construye el cliente, repositorio y caso de uso.
func NewSubjectJobsContainer(ctx context.Context, config Config) (*SubjectJobsContainer, error) {
	if !config.SubjectBootstrapEnabled && !config.SubjectProjectionEnabled && !config.SubjectContextRegistryEnabled {
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
	if config.SubjectContextRegistryEnabled {
		redisClient := goredis.NewClient(&goredis.Options{Addr: config.RedisAddr, Username: config.RedisUsername, Password: config.RedisPassword, DB: config.RedisDB})
		if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
			pool.Close()
			_ = redisClient.Close()
			return nil, pingErr
		}
		cache := redisadapter.NewContextRegistryCache(redisClient, config.SubjectContextRegistryTTL, config.SubjectContextRegistryLockTTL)
		repository, repositoryErr := postgresadapter.NewContextRegistryRepository(pool)
		if repositoryErr != nil {
			pool.Close()
			_ = redisClient.Close()
			return nil, repositoryErr
		}
		source, sourceErr := httpregistry.NewClient(config.SubjectContextRegistryURL, config.SubjectContextRegistryToken, config.SubjectContextRegistryTimeout)
		if sourceErr != nil {
			pool.Close()
			_ = redisClient.Close()
			return nil, sourceErr
		}
		container.Redis = redisClient
		container.Registry = usecases.NewContextRegistryLoader(cache, cache, repository, source)
		if config.SubjectProjectionEnabled {
			projectionRepository, projectionErr := postgresadapter.NewProjectionRepository(pool)
			if projectionErr != nil {
				pool.Close()
				_ = redisClient.Close()
				return nil, projectionErr
			}
			container.ProjectContext, err = usecases.NewProjectContext(projectionRepository, config.SubjectProjectionVersion, config.SubjectProjectionBatchSize)
			if err != nil {
				pool.Close()
				_ = redisClient.Close()
				return nil, err
			}
			container.Scheduler = usecases.NewProjectionScheduler(projectionRepository, usecases.NewProjectionSequence(container.ProjectGlobal, container.ProjectContext), config.SubjectProjectionVersion, config.SubjectProjectionInterval, config.SubjectJobsCheckInterval, "subject_global", "subject_context")
		}
		resolutionRepository, resolutionErr := postgresadapter.NewContextResolutionRepository(pool)
		if resolutionErr != nil {
			pool.Close()
			_ = redisClient.Close()
			return nil, resolutionErr
		}
		container.ResolveInitial, err = usecases.NewResolveInitialContexts(resolutionRepository, container.Registry, config.SubjectProjectionBatchSize)
		if err != nil {
			pool.Close()
			_ = redisClient.Close()
			return nil, err
		}
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
	if container != nil && container.Redis != nil {
		_ = container.Redis.Close()
	}
}
