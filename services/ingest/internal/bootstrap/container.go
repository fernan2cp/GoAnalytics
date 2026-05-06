package bootstrap

import (
	"context"
	"fmt"
	nethttp "net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"

	inboundhttp "goanalytics/services/ingest/internal/adapters/inbound/http"
	jwtadapter "goanalytics/services/ingest/internal/adapters/outbound/jwt"
	loggeradapter "goanalytics/services/ingest/internal/adapters/outbound/logger"
	redisadapter "goanalytics/services/ingest/internal/adapters/outbound/redis"
	"goanalytics/services/ingest/internal/adapters/outbound/system"
	"goanalytics/services/ingest/internal/application/ports/outbound"
	"goanalytics/services/ingest/internal/application/usecases"
)

// Container agrupa dependencias vivas del servicio de ingesta.
//
// Contiene configuracion, cliente Redis, logger y router HTTP. Debe crearse en
// main y cerrarse al finalizar el proceso para liberar conexiones.
type Container struct {
	Config      Config
	RedisClient *goredis.Client
	Logger      outbound.Logger
	Handler     nethttp.Handler
}

// NewContainer inicializa adaptadores y caso de uso de ingesta.
//
// Recibe configuracion ya cargada y devuelve un Container listo para iniciar
// el servidor HTTP. Devuelve error si JWT, Redis o publisher no pueden
// construirse.
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

	tokenVerifier, err := jwtadapter.NewHS256Verifier(config.GoAnalyticsJWTSecret, config.JWTIssuer, config.JWTAudience, config.JWTMaxLifetime())
	if err != nil {
		return nil, err
	}
	publisher, err := redisadapter.NewEventStreamPublisher(redisClient, config.EventStreamName, config.EventStreamMaxLen)
	if err != nil {
		return nil, err
	}

	var limiter outbound.RateLimiter
	if config.RateLimitEnabled {
		limiter, err = redisadapter.NewRateLimiter(redisClient, "goanalytics:ratelimit")
		if err != nil {
			return nil, err
		}
	} else {
		limiter = redisadapter.NoopRateLimiter{}
	}

	useCase := usecases.NewIngestEventsUseCase(
		tokenVerifier,
		publisher,
		limiter,
		system.Clock{},
		appLogger,
		usecases.IngestOptions{
			MaxEventsPerBatch: config.MaxEventsPerBatch,
			SiteRateLimit:     config.RateLimitEventsPerMinuteSite,
			IPRateLimit:       config.RateLimitEventsPerMinuteIP,
			RateLimitWindow:   time.Minute,
			SDKName:           "goanalytics-web",
			SDKVersion:        "0.1.0",
		},
	)
	eventsHandler := inboundhttp.NewIngestHandler(useCase, appLogger, config.MaxEventPayloadBytes, config.HideAuthFailures)
	readyHandler := inboundhttp.NewReadyHandler(func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})

	return &Container{
		Config:      config,
		RedisClient: redisClient,
		Logger:      appLogger,
		Handler:     inboundhttp.NewRouter(eventsHandler, inboundhttp.NewHealthHandler(), readyHandler),
	}, nil
}

// Close libera recursos abiertos por el container.
//
// No recibe parametros y devuelve error cuando el cierre del cliente Redis
// falla. Debe llamarse desde main al detener el proceso.
func (container *Container) Close() error {
	if container == nil || container.RedisClient == nil {
		return nil
	}
	return container.RedisClient.Close()
}
