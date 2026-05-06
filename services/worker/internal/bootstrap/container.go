package bootstrap

import (
	"context"
	"fmt"
	nethttp "net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/worker/internal/adapters/inbound/httphealth"
	"goanalytics/services/worker/internal/adapters/inbound/redisstream"
	"goanalytics/services/worker/internal/adapters/outbound/httpresolver"
	loggeradapter "goanalytics/services/worker/internal/adapters/outbound/logger"
	postgresadapter "goanalytics/services/worker/internal/adapters/outbound/postgres"
	redisadapter "goanalytics/services/worker/internal/adapters/outbound/redis"
	"goanalytics/services/worker/internal/adapters/outbound/system"
	"goanalytics/services/worker/internal/application/ports/inbound"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/application/usecases"
)

// Container agrupa dependencias vivas del worker.
//
// Contiene configuracion, clientes Redis/PostgreSQL, logger y consumer. Debe
// crearse en main y cerrarse al finalizar el proceso para liberar conexiones.
type Container struct {
	Config      Config
	RedisClient *goredis.Client
	Postgres    *pgxpool.Pool
	Logger      outbound.Logger
	Consumer    inbound.EventConsumer
	Health      nethttp.Handler
}

// NewContainer inicializa adaptadores y caso de uso del worker.
//
// Recibe configuracion ya cargada y devuelve un Container listo para ejecutar
// el consumer. Devuelve error si Redis, PostgreSQL, resolver o repositorios no
// pueden construirse.
func NewContainer(ctx context.Context, config Config) (*Container, error) {
	appLogger := loggeradapter.NewSlogLogger(config.LogLevel)
	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     config.RedisAddr,
		Username: config.RedisUsername,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis no disponible: %w", err)
	}

	pgPool, err := pgxpool.New(ctx, config.PostgresDSN())
	if err != nil {
		return nil, fmt.Errorf("postgres no configurable: %w", err)
	}
	if err := pgPool.Ping(ctx); err != nil {
		pgPool.Close()
		return nil, fmt.Errorf("postgres no disponible: %w", err)
	}

	siteCache, err := redisadapter.NewSiteCache(redisClient, config.SiteNegativeCacheTTL)
	if err != nil {
		return nil, err
	}
	rehydrationGate, err := redisadapter.NewRehydrationGate(redisClient, config.SiteRehydrateCooldown)
	if err != nil {
		return nil, err
	}
	siteResolver, err := httpresolver.NewSiteResolver(
		config.SiteResolverURL,
		config.SiteResolverToken,
		config.SiteResolverTimeout,
		rehydrationGate,
		siteCache,
	)
	if err != nil {
		return nil, err
	}
	eventRepository, err := postgresadapter.NewEventRepository(pgPool)
	if err != nil {
		return nil, err
	}
	rejectedRepository, err := postgresadapter.NewRejectedEventRepository(pgPool)
	if err != nil {
		return nil, err
	}
	deduplicator, err := redisadapter.NewDeduplicator(redisClient, config.DeduplicationTTL)
	if err != nil {
		return nil, err
	}

	process := usecases.NewProcessEventsUseCaseWithOptions(
		nil,
		eventRepository,
		rejectedRepository,
		siteCache,
		siteResolver,
		deduplicator,
		system.Clock{},
		appLogger,
		usecases.RehydrateSiteOptions{SiteCacheTTL: config.SiteCacheTTL},
	)
	consumer, err := redisstream.NewConsumer(redisClient, redisstream.Config{
		StreamName:   config.EventStreamName,
		GroupName:    config.WorkerConsumerGroup,
		ConsumerName: config.WorkerName,
		BatchSize:    config.WorkerBatchSize,
		PollInterval: config.WorkerPollInterval,
	}, process, appLogger)
	if err != nil {
		return nil, err
	}
	readyHandler := httphealth.NewReadyHandler(func(ctx context.Context) error {
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return err
		}
		return pgPool.Ping(ctx)
	})

	return &Container{
		Config:      config,
		RedisClient: redisClient,
		Postgres:    pgPool,
		Logger:      appLogger,
		Consumer:    consumer,
		Health:      httphealth.NewRouter(httphealth.NewHealthHandler(), readyHandler),
	}, nil
}

// Close libera recursos abiertos por el container.
//
// No recibe parametros y devuelve error cuando el cierre de Redis falla.
// PostgreSQL no devuelve error al cerrar su pool.
func (container *Container) Close() error {
	if container == nil {
		return nil
	}
	if container.Postgres != nil {
		container.Postgres.Close()
	}
	if container.RedisClient != nil {
		return container.RedisClient.Close()
	}
	return nil
}
