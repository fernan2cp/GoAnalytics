package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"goanalytics/services/ingest/internal/application/dto"
	"goanalytics/services/ingest/internal/application/ports/outbound"
	"goanalytics/services/ingest/internal/domain/event"
)

// Errores de aplicacion devueltos por el caso de uso de ingesta.
//
// Permiten que los adaptadores inbound traduzcan fallas a respuestas externas
// sin conocer detalles de dominio ni de infraestructura concreta.
var (
	ErrInvalidToken      = errors.New("token de ingesta invalido")
	ErrInvalidPayload    = errors.New("payload de ingesta invalido")
	ErrInvalidBatch      = errors.New("batch de ingesta invalido")
	ErrRateLimitExceeded = errors.New("limite de ingesta excedido")
	ErrPublishFailed     = errors.New("publicacion de eventos fallida")
	ErrDependencyMissing = errors.New("dependencia de ingesta faltante")
)

// IngestOptions define la politica configurable del caso de uso de ingesta.
//
// Recibe limites de batch, limites por site e IP, ventana de rate limit y
// metadatos por defecto del SDK. La estructura se inyecta desde bootstrap para
// evitar que application lea variables de entorno.
//
// Los limites de rate limit con valor cero o negativo desactivan esa dimension.
// MaxEventsPerBatch con valor cero o negativo usa DefaultMaxEventsPerBatch.
type IngestOptions struct {
	MaxEventsPerBatch int
	SiteRateLimit     int
	IPRateLimit       int
	RateLimitWindow   time.Duration
	SDKName           string
	SDKVersion        string
}

const (
	// DefaultMaxEventsPerBatch es el maximo conservador para Fase 1.
	DefaultMaxEventsPerBatch = 50
	defaultRateLimitWindow   = time.Minute
)

// IngestEventsUseCase orquesta la aceptacion inicial de eventos.
//
// Usa puertos de salida para validar el token, aplicar rate limit, publicar
// eventos crudos, obtener tiempo y registrar logs. Es un tipo de aplicacion y
// no debe importar HTTP, Redis, JWT concreto ni variables de entorno.
//
// Debe construirse con NewIngestEventsUseCase. Sus metodos devuelven error
// cuando una validacion o dependencia impide aceptar la solicitud.
type IngestEventsUseCase struct {
	tokenVerifier outbound.EventTokenVerifier
	publisher     outbound.EventPublisher
	rateLimiter   outbound.RateLimiter
	clock         outbound.Clock
	logger        outbound.Logger
	options       IngestOptions
}

// NewIngestEventsUseCase crea el caso de uso de ingesta de eventos.
//
// Recibe puertos de salida para verificar tokens, publicar eventos, aplicar
// limites, obtener tiempo, registrar logs y una politica de opciones. Devuelve
// una instancia preparada para orquestar la ingesta.
//
// Debe usarse desde bootstrap con adaptadores ya inicializados. No devuelve
// error porque solo asigna dependencias; las fallas de infraestructura se
// informan cuando se ejecuta Ingest. Las opciones se normalizan para asegurar
// defaults testeables sin leer variables de entorno.
func NewIngestEventsUseCase(
	tokenVerifier outbound.EventTokenVerifier,
	publisher outbound.EventPublisher,
	rateLimiter outbound.RateLimiter,
	clock outbound.Clock,
	logger outbound.Logger,
	options IngestOptions,
) *IngestEventsUseCase {
	return &IngestEventsUseCase{
		tokenVerifier: tokenVerifier,
		publisher:     publisher,
		rateLimiter:   rateLimiter,
		clock:         clock,
		logger:        logger,
		options:       normalizeOptions(options),
	}
}

// Ingest procesa una solicitud de ingesta.
//
// Recibe un contexto de ejecucion y un dto.IngestRequest con token, datos del
// cliente HTTP y eventos crudos del SDK. Devuelve dto.IngestResponse con la
// cantidad aceptada para procesamiento.
//
// Valida el token, comprueba claims de dominio, aplica limites, transforma los
// DTOs a eventos crudos enriquecidos y los publica por el puerto outbound.
// Devuelve error cuando falla una validacion, un limite o una dependencia.
func (uc *IngestEventsUseCase) Ingest(ctx context.Context, request dto.IngestRequest) (dto.IngestResponse, error) {
	if err := uc.ensureDependencies(); err != nil {
		return dto.IngestResponse{}, err
	}
	if strings.TrimSpace(request.Token) == "" {
		return dto.IngestResponse{}, ErrInvalidToken
	}
	if len(request.Events) == 0 {
		return dto.IngestResponse{}, ErrInvalidBatch
	}
	if len(request.Events) > uc.options.MaxEventsPerBatch {
		return dto.IngestResponse{}, fmt.Errorf("%w: maximo %d", ErrInvalidBatch, uc.options.MaxEventsPerBatch)
	}

	now := uc.clock.Now()
	claims, err := uc.tokenVerifier.Verify(ctx, request.Token)
	if err != nil {
		return dto.IngestResponse{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if err := claims.Validate(now); err != nil {
		return dto.IngestResponse{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if err := uc.applyRateLimits(ctx, claims.SiteCode, request.IPHash); err != nil {
		return dto.IngestResponse{}, err
	}

	rawEvents := make([]event.RawEvent, 0, len(request.Events))
	eventIDs := make([]string, 0, len(request.Events))
	rejected := 0
	for _, item := range request.Events {
		raw := event.RawEvent{
			EventID:                item.EventID,
			LogicalEventID:         item.LogicalEventID,
			IdempotencyKey:         item.IdempotencyKey,
			TabID:                  item.TabID,
			Sequence:               item.Sequence,
			PreviousLogicalEventID: item.PreviousLogicalEventID,
			SiteCode:               claims.SiteCode,
			Environment:            claims.Environment,
			TokenVersion:           claims.TokenVersion,
			JWTID:                  claims.JWTID,
			EventName:              item.EventName,
			EventVersion:           item.EventVersion,
			EventTime:              item.Timestamp,
			ReceivedAt:             now,
			AnonymousID:            item.AnonymousID,
			SessionID:              item.SessionID,
			UserID:                 item.UserID,
			Origin:                 item.Origin,
			URL:                    item.URL,
			Path:                   item.Path,
			Referrer:               item.Referrer,
			UserAgent:              request.UserAgent,
			IPHash:                 request.IPHash,
			SDKName:                uc.options.SDKName,
			SDKVersion:             uc.options.SDKVersion,
			Properties:             event.NormalizeMap(item.Properties),
			Context:                event.NormalizeMap(item.Context),
		}
		if err := event.ValidateRawEvent(raw); err != nil {
			rejected++
			uc.warn(ctx, "evento de ingesta rechazado por payload invalido", map[string]any{
				"event_id":   item.EventID,
				"event_name": item.EventName,
				"reason":     err.Error(),
			})
			rawEvents = append(rawEvents, raw)
			continue
		}
		rawEvents = append(rawEvents, raw)
		eventIDs = append(eventIDs, raw.EventID)
	}

	if len(rawEvents) > 0 {
		if err := uc.publisher.Publish(ctx, rawEvents); err != nil {
			return dto.IngestResponse{}, fmt.Errorf("%w: %v", ErrPublishFailed, err)
		}
	}
	return dto.IngestResponse{Accepted: len(eventIDs), Rejected: rejected, EventIDs: eventIDs}, nil
}

// warn registra una advertencia si el logger esta configurado.
//
// Recibe contexto, mensaje y atributos. No devuelve error porque el log no debe
// cambiar el resultado de la ingesta.
func (uc *IngestEventsUseCase) warn(ctx context.Context, message string, attrs map[string]any) {
	if uc.logger == nil {
		return
	}
	uc.logger.Warn(ctx, message, attrs)
}

// normalizeOptions aplica defaults de aplicacion a la politica de ingesta.
//
// Recibe opciones posiblemente parciales y devuelve una copia completa. No
// valida variables de entorno ni detalles de adaptadores.
func normalizeOptions(options IngestOptions) IngestOptions {
	if options.MaxEventsPerBatch <= 0 {
		options.MaxEventsPerBatch = DefaultMaxEventsPerBatch
	}
	if options.RateLimitWindow <= 0 {
		options.RateLimitWindow = defaultRateLimitWindow
	}
	return options
}

// ensureDependencies confirma que el caso de uso tenga puertos requeridos.
//
// No recibe parametros y devuelve ErrDependencyMissing cuando falta un puerto
// necesario para ejecutar la ingesta. Evita panics en tests y bootstrap.
func (uc *IngestEventsUseCase) ensureDependencies() error {
	if uc == nil || uc.tokenVerifier == nil || uc.publisher == nil || uc.rateLimiter == nil || uc.clock == nil {
		return ErrDependencyMissing
	}
	return nil
}

// applyRateLimits aplica limites por site e IP segun las opciones inyectadas.
//
// Recibe identidad publica de site e IP hasheada. Devuelve ErrInvalidPayload
// cuando falta una clave necesaria, ErrRateLimitExceeded cuando el limite se
// agota o el error de infraestructura devuelto por el puerto.
func (uc *IngestEventsUseCase) applyRateLimits(ctx context.Context, siteCode string, ipHash string) error {
	if uc.options.SiteRateLimit > 0 {
		allowed, err := uc.rateLimiter.Allow(ctx, "site:"+siteCode, uc.options.SiteRateLimit, uc.options.RateLimitWindow)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRateLimitExceeded
		}
	}
	if uc.options.IPRateLimit > 0 {
		if strings.TrimSpace(ipHash) == "" {
			return fmt.Errorf("%w: ip_hash requerido para rate limit", ErrInvalidPayload)
		}
		allowed, err := uc.rateLimiter.Allow(ctx, "ip:"+ipHash, uc.options.IPRateLimit, uc.options.RateLimitWindow)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRateLimitExceeded
		}
	}
	return nil
}
