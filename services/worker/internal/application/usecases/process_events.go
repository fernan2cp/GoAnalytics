package usecases

import (
	"context"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/application/ports/outbound"
)

// ProcessEventsUseCase orquesta el procesamiento de eventos del worker.
//
// Usa puertos de salida para consumir eventos, validar metadata, resolver
// rehidratacion, deduplicar, persistir eventos validos y registrar rechazos.
// Es un tipo de aplicacion y no debe importar Redis, PostgreSQL, HTTP ni
// variables de entorno.
//
// Debe construirse con NewProcessEventsUseCase. Sus metodos devuelven error
// cuando una dependencia o validacion impide completar el batch.
type ProcessEventsUseCase struct {
	consumer          outbound.EventConsumer
	eventRepository   outbound.EventRepository
	rejectedRepository outbound.RejectedEventRepository
	siteCache         outbound.SiteCache
	siteResolver      outbound.SiteResolver
	deduplicator      outbound.Deduplicator
	clock             outbound.Clock
	logger            outbound.Logger
}

// NewProcessEventsUseCase crea el caso de uso de procesamiento de eventos.
//
// Recibe puertos de salida para consumir eventos, persistir eventos validos,
// registrar rechazos, consultar metadata de site, resolver rehidratacion,
// deduplicar, obtener tiempo y registrar logs. Devuelve una instancia lista
// para procesar batches.
//
// Debe usarse desde bootstrap con adaptadores ya inicializados. No devuelve
// error porque solo asigna dependencias; las fallas de infraestructura se
// informan cuando se ejecuta Process.
func NewProcessEventsUseCase(
	consumer outbound.EventConsumer,
	eventRepository outbound.EventRepository,
	rejectedRepository outbound.RejectedEventRepository,
	siteCache outbound.SiteCache,
	siteResolver outbound.SiteResolver,
	deduplicator outbound.Deduplicator,
	clock outbound.Clock,
	logger outbound.Logger,
) *ProcessEventsUseCase {
	return &ProcessEventsUseCase{
		consumer:          consumer,
		eventRepository:   eventRepository,
		rejectedRepository: rejectedRepository,
		siteCache:         siteCache,
		siteResolver:      siteResolver,
		deduplicator:      deduplicator,
		clock:             clock,
		logger:            logger,
	}
}

// Process procesa un batch de eventos crudos del stream.
//
// Recibe un contexto de ejecucion y una lista de dto.RawEvent previamente
// consumidos por un adaptador. No devuelve datos; devuelve error si el
// procesamiento o alguna dependencia falla.
//
// En esta fase inicial solo conserva la firma del caso de uso. En fases
// posteriores validara site, dominio, token_version, deduplicacion,
// persistencia y registro de rechazos.
func (uc *ProcessEventsUseCase) Process(ctx context.Context, events []dto.RawEvent) error {
	_ = uc
	_ = ctx
	_ = events
	return nil
}
