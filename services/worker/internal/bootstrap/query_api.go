package bootstrap

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/adapters/inbound/httphealth"
	"goanalytics/services/worker/internal/adapters/inbound/httpsuggestions"
	metricsadapter "goanalytics/services/worker/internal/adapters/outbound/metrics"
	postgresadapter "goanalytics/services/worker/internal/adapters/outbound/postgres"
	"goanalytics/services/worker/internal/application/usecases"
)

// QueryAPIContainer agrupa PostgreSQL y el router HTTP de consultas.
type QueryAPIContainer struct {
	Postgres *pgxpool.Pool
	Handler  http.Handler
}

// NewQueryAPIContainer construye el proceso de lectura sin Redis Stream.
func NewQueryAPIContainer(ctx context.Context, config Config) (*QueryAPIContainer, error) {
	pool, err := pgxpool.New(ctx, config.PostgresDSN())
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	repository, err := postgresadapter.NewSuggestionRepository(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	query := usecases.NewQuerySuggestions(repository, config.SubjectProjectionVersion, config.SubjectSuggestionGlobalDays, config.SubjectSuggestionContextDays, config.SubjectSuggestionStaleAfter, config.SubjectSuggestionServeStale)
	metrics := metricsadapter.NewRegistry()
	suggestions := httpsuggestions.NewHandler(config.SubjectQueryAPIEnabled, config.SubjectQueryAPIToken, config.SubjectSuggestionDefaultLimit, config.SubjectSuggestionMaxLimit, query, metrics)
	operations := httphealth.NewRouterWithMetrics(httphealth.NewHealthHandler(), httphealth.NewReadyHandler(pool.Ping), metrics.Handler())
	mux := http.NewServeMux()
	mux.Handle("/health", operations)
	mux.Handle("/ready", operations)
	mux.Handle("/metrics", operations)
	mux.Handle("/v1/internal/subjects/suggestions", suggestions)
	return &QueryAPIContainer{Postgres: pool, Handler: mux}, nil
}

// Close libera PostgreSQL.
func (container *QueryAPIContainer) Close() {
	if container != nil && container.Postgres != nil {
		container.Postgres.Close()
	}
}
