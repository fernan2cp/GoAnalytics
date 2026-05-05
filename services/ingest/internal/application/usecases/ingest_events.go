package usecases

import (
	"context"

	"goanalytics/services/ingest/internal/application/dto"
	"goanalytics/services/ingest/internal/application/ports/outbound"
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
}

// NewIngestEventsUseCase crea el caso de uso de ingesta de eventos.
//
// Recibe puertos de salida para verificar tokens, publicar eventos, aplicar
// limites, obtener tiempo y registrar logs. Devuelve una instancia preparada
// para orquestar la ingesta.
//
// Debe usarse desde bootstrap con adaptadores ya inicializados. No devuelve
// error porque solo asigna dependencias; las fallas de infraestructura se
// informan cuando se ejecuta Ingest.
func NewIngestEventsUseCase(
	tokenVerifier outbound.EventTokenVerifier,
	publisher outbound.EventPublisher,
	rateLimiter outbound.RateLimiter,
	clock outbound.Clock,
	logger outbound.Logger,
) *IngestEventsUseCase {
	return &IngestEventsUseCase{
		tokenVerifier: tokenVerifier,
		publisher:     publisher,
		rateLimiter:   rateLimiter,
		clock:         clock,
		logger:        logger,
	}
}

// Ingest procesa una solicitud de ingesta.
//
// Recibe un contexto de ejecucion y un dto.IngestRequest con token, datos del
// cliente HTTP y eventos crudos del SDK. Devuelve dto.IngestResponse con la
// cantidad aceptada para procesamiento.
//
// En esta fase inicial solo conserva la firma del caso de uso y retorna la
// cantidad de eventos recibidos. En fases posteriores validara JWT, limites,
// estructura y publicacion. Devolvera error cuando falle una validacion o un
// puerto de salida requerido.
func (uc *IngestEventsUseCase) Ingest(ctx context.Context, request dto.IngestRequest) (dto.IngestResponse, error) {
	_ = uc
	_ = ctx
	return dto.IngestResponse{Accepted: len(request.Events)}, nil
}
