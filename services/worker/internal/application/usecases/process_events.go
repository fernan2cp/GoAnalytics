package usecases

import (
	"context"
	"errors"
	"fmt"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/rejection"
	"goanalytics/services/worker/internal/domain/site"
)

// Errores de aplicacion devueltos por el procesamiento del worker.
//
// Permiten distinguir dependencias faltantes y fallas de persistencia sin
// exponer detalles de Redis, PostgreSQL u otros adaptadores.
var (
	ErrProcessDependencyMissing = errors.New("dependencia de procesamiento faltante")
	ErrPersistValidEventsFailed = errors.New("persistencia de eventos validos fallida")
	ErrPersistRejectedFailed    = errors.New("persistencia de rechazos fallida")
	ErrDeduplicationFailed      = errors.New("deduplicacion fallida")
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
	consumer           outbound.EventConsumer
	eventRepository    outbound.EventRepository
	rejectedRepository outbound.RejectedEventRepository
	siteCache          outbound.SiteCache
	siteResolver       outbound.SiteResolver
	deduplicator       outbound.Deduplicator
	clock              outbound.Clock
	logger             outbound.Logger
	validateSite       *ValidateSiteUseCase
	semanticRules      []SemanticDedupRule
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
	return NewProcessEventsUseCaseWithOptions(
		consumer,
		eventRepository,
		rejectedRepository,
		siteCache,
		siteResolver,
		deduplicator,
		clock,
		logger,
		RehydrateSiteOptions{},
		nil,
	)
}

// NewProcessEventsUseCaseWithOptions crea el caso de uso con opciones.
//
// Recibe los mismos puertos que NewProcessEventsUseCase y opciones de
// rehidratacion para ajustar TTL de cache desde bootstrap. Devuelve una
// instancia lista para procesar batches.
func NewProcessEventsUseCaseWithOptions(
	consumer outbound.EventConsumer,
	eventRepository outbound.EventRepository,
	rejectedRepository outbound.RejectedEventRepository,
	siteCache outbound.SiteCache,
	siteResolver outbound.SiteResolver,
	deduplicator outbound.Deduplicator,
	clock outbound.Clock,
	logger outbound.Logger,
	rehydrateOptions RehydrateSiteOptions,
	semanticRules []SemanticDedupRule,
) *ProcessEventsUseCase {
	rehydrateSite := NewRehydrateSiteUseCase(siteResolver, siteCache, rehydrateOptions)
	return &ProcessEventsUseCase{
		consumer:           consumer,
		eventRepository:    eventRepository,
		rejectedRepository: rejectedRepository,
		siteCache:          siteCache,
		siteResolver:       siteResolver,
		deduplicator:       deduplicator,
		clock:              clock,
		logger:             logger,
		validateSite:       NewValidateSiteUseCase(siteCache, rehydrateSite),
		semanticRules:      semanticRules,
	}
}

// Process procesa un batch de eventos crudos del stream.
//
// Recibe un contexto de ejecucion y una lista de dto.RawEvent previamente
// consumidos por un adaptador. No devuelve datos; devuelve error si el
// procesamiento o alguna dependencia falla.
func (uc *ProcessEventsUseCase) Process(ctx context.Context, events []dto.RawEvent) error {
	if err := uc.ensureDependencies(); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	validEvents := make([]event.ValidatedEvent, 0, len(events))
	rejectedEvents := make([]rejection.RejectedEvent, 0)

	for _, raw := range events {
		if err := validateRawEvent(raw); err != nil {
			rejectedEvents = append(rejectedEvents, uc.buildRejectedEvent(raw, rejection.ReasonInvalidPayload, rejection.SeverityWarning, ""))
			continue
		}
		if blockedKey := blockedPayloadKey(raw); blockedKey != "" {
			rejected := uc.buildRejectedEvent(raw, rejection.ReasonBlockedKey, rejection.SeverityWarning, "")
			rejected.RawPayload["blocked_key"] = blockedKey
			rejectedEvents = append(rejectedEvents, rejected)
			continue
		}

		exact := exactDedupCandidate(raw)
		seen, err := uc.deduplicator.Seen(ctx, exact.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrDeduplicationFailed, err)
		}
		if seen {
			rejectedEvents = append(rejectedEvents, uc.buildRejectedEvent(raw, rejection.ReasonDuplicateEvent, rejection.SeverityWarning, exact.Strategy))
			continue
		}

		config, err := uc.validateSite.Validate(ctx, raw)
		if err != nil {
			rejectedEvents = append(rejectedEvents, uc.buildRejectedEvent(raw, rejectionReason(err), rejectionSeverity(err), ""))
			continue
		}
		dedupStrategy, duplicateReason, err := uc.evaluateStrongAndSemanticDedup(ctx, raw, config)
		if err != nil {
			return err
		}
		if duplicateReason != "" {
			rejectedEvents = append(rejectedEvents, uc.buildRejectedEvent(raw, duplicateReason, rejection.SeverityWarning, dedupStrategy))
			continue
		}
		validEvent := buildValidatedEvent(raw, config, dedupStrategy)
		validEvent.ItemDetails = buildItemDetails(validEvent)
		validEvent.OrderDetail = buildOrderDetail(validEvent)
		itemDedupStrategy, itemDuplicateReason, err := uc.evaluateItemSpecificDedup(ctx, validEvent)
		if err != nil {
			return err
		}
		if itemDuplicateReason != "" {
			rejectedEvents = append(rejectedEvents, uc.buildRejectedEvent(raw, itemDuplicateReason, rejection.SeverityWarning, itemDedupStrategy))
			continue
		}
		validEvents = append(validEvents, validEvent)
	}

	if len(rejectedEvents) > 0 {
		if err := uc.rejectedRepository.SaveBatch(ctx, rejectedEvents); err != nil {
			return fmt.Errorf("%w: %v", ErrPersistRejectedFailed, err)
		}
	}
	if len(validEvents) > 0 {
		if err := uc.eventRepository.SaveBatch(ctx, validEvents); err != nil {
			return fmt.Errorf("%w: %v", ErrPersistValidEventsFailed, err)
		}
		for _, valid := range validEvents {
			if err := uc.markDeduplicationKeys(ctx, valid); err != nil {
				return fmt.Errorf("%w: %v", ErrDeduplicationFailed, err)
			}
		}
	}
	return nil
}

// ensureDependencies confirma que el caso de uso tenga puertos requeridos.
//
// No recibe parametros y devuelve ErrProcessDependencyMissing si falta un
// puerto necesario. Evita panics en tests y bootstrap incompleto.
func (uc *ProcessEventsUseCase) ensureDependencies() error {
	if uc == nil ||
		uc.eventRepository == nil ||
		uc.rejectedRepository == nil ||
		uc.siteCache == nil ||
		uc.siteResolver == nil ||
		uc.deduplicator == nil ||
		uc.clock == nil ||
		uc.validateSite == nil {
		return ErrProcessDependencyMissing
	}
	return nil
}

// validateRawEvent aplica las reglas minimas del evento crudo.
//
// Recibe dto.RawEvent y devuelve error de dominio si falta algun campo
// requerido para procesamiento. No consulta cache, resolver ni repositorios.
func validateRawEvent(raw dto.RawEvent) error {
	return event.ValidateRequiredFields(
		raw.EventID,
		raw.SiteCode,
		raw.Environment,
		raw.TokenVersion,
		raw.JWTID,
		raw.EventName,
		raw.EventVersion,
		raw.EventTime.IsZero(),
		raw.AnonymousID,
		raw.SessionID,
		raw.Origin,
		raw.URL,
		raw.Path,
	)
}

// buildValidatedEvent combina evento crudo y metadata real de site.
//
// Recibe dto.RawEvent y site.SiteConfig ya validados. Devuelve
// event.ValidatedEvent listo para persistir mediante EventRepository.
func buildValidatedEvent(raw dto.RawEvent, config site.SiteConfig, dedupStrategy string) event.ValidatedEvent {
	return event.ValidatedEvent{
		EventID:                raw.EventID,
		LogicalEventID:         raw.LogicalEventID,
		IdempotencyKey:         raw.IdempotencyKey,
		TabID:                  raw.TabID,
		Sequence:               raw.Sequence,
		PreviousLogicalEventID: raw.PreviousLogicalEventID,
		DedupStrategy:          dedupStrategy,
		TenantID:               config.TenantID,
		SiteID:                 config.SiteID,
		SiteCode:               raw.SiteCode,
		Environment:            raw.Environment,
		EventName:              raw.EventName,
		EventVersion:           raw.EventVersion,
		EventTime:              raw.EventTime,
		ReceivedAt:             raw.ReceivedAt,
		AnonymousID:            raw.AnonymousID,
		UserID:                 raw.UserID,
		SessionID:              raw.SessionID,
		Origin:                 raw.Origin,
		URL:                    raw.URL,
		Path:                   raw.Path,
		Referrer:               raw.Referrer,
		UserAgent:              raw.UserAgent,
		IPHash:                 raw.IPHash,
		SDKName:                raw.SDKName,
		SDKVersion:             raw.SDKVersion,
		JWTID:                  raw.JWTID,
		TokenVersion:           raw.TokenVersion,
		Properties:             event.NormalizeMap(raw.Properties),
		Context:                event.NormalizeMap(raw.Context),
	}
}

// buildRejectedEvent arma un rechazo auditable para un evento crudo.
//
// Recibe dto.RawEvent, motivo y severidad. Devuelve rejection.RejectedEvent con
// created_at tomado del reloj inyectado y payload minimo no sensible.
func (uc *ProcessEventsUseCase) buildRejectedEvent(raw dto.RawEvent, reason string, severity string, dedupStrategy string) rejection.RejectedEvent {
	return rejection.RejectedEvent{
		EventID:     raw.EventID,
		SiteCode:    raw.SiteCode,
		Environment: raw.Environment,
		Reason:      reason,
		Severity:    severity,
		Origin:      raw.Origin,
		URL:         raw.URL,
		IPHash:      raw.IPHash,
		UserAgent:   raw.UserAgent,
		RawPayload: map[string]any{
			"event_id":                  raw.EventID,
			"logical_event_id":          raw.LogicalEventID,
			"idempotency_key":           raw.IdempotencyKey,
			"tab_id":                    raw.TabID,
			"sequence":                  raw.Sequence,
			"previous_logical_event_id": raw.PreviousLogicalEventID,
			"event_name":                raw.EventName,
			"event_version":             raw.EventVersion,
			"path":                      raw.Path,
			"dedup_strategy":            dedupStrategy,
		},
		CreatedAt: uc.clock.Now(),
	}
}

// evaluateStrongAndSemanticDedup aplica capas no exactas de deduplicacion.
//
// Recibe evento crudo y metadata validada. Devuelve la estrategia elegida, una
// razon de duplicado si corresponde y error si falla la infraestructura.
func (uc *ProcessEventsUseCase) evaluateStrongAndSemanticDedup(ctx context.Context, raw dto.RawEvent, config site.SiteConfig) (string, string, error) {
	strong := strongDedupCandidate(raw, config)
	if !strong.Empty {
		seen, err := uc.deduplicator.Seen(ctx, strong.Key)
		if err != nil {
			return "", "", fmt.Errorf("%w: %v", ErrDeduplicationFailed, err)
		}
		if seen {
			if strong.Strategy == dedupStrategyIdempotency {
				return strong.Strategy, rejection.ReasonDuplicateLogicalEvent, nil
			}
			return strong.Strategy, rejection.ReasonDuplicateLogicalEvent, nil
		}
		return strong.Strategy, "", nil
	}
	semantic := semanticDedupCandidate(raw, config, uc.semanticRules)
	if semantic.Empty {
		return dedupStrategyNone, "", nil
	}
	seen, err := uc.deduplicator.Seen(ctx, semantic.Key)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrDeduplicationFailed, err)
	}
	if seen {
		return semantic.Strategy, rejection.ReasonDuplicateSemanticEvent, nil
	}
	return semantic.Strategy, "", nil
}

// evaluateItemSpecificDedup aplica deduplicacion especializada por detalle de item.
func (uc *ProcessEventsUseCase) evaluateItemSpecificDedup(ctx context.Context, valid event.ValidatedEvent) (string, string, error) {
	keys := itemSpecificDedupKeys(valid)
	for _, key := range keys {
		seen, err := uc.deduplicator.Seen(ctx, key)
		if err != nil {
			return "", "", fmt.Errorf("%w: %v", ErrDeduplicationFailed, err)
		}
		if seen {
			return key.Strategy, rejection.ReasonDuplicateSemanticEvent, nil
		}
	}
	return "", "", nil
}

// markDeduplicationKeys registra las claves vistas despues de persistir.
func (uc *ProcessEventsUseCase) markDeduplicationKeys(ctx context.Context, valid event.ValidatedEvent) error {
	keys := []outbound.DeduplicationKey{
		{Strategy: dedupStrategyExact, Key: valid.EventID},
	}
	config := site.SiteConfig{TenantID: valid.TenantID, SiteID: valid.SiteID}
	if valid.IdempotencyKey != "" {
		keys = append(keys, outbound.DeduplicationKey{Strategy: dedupStrategyIdempotency, Key: scopedKey(config, valid.IdempotencyKey)})
	}
	if valid.LogicalEventID != "" {
		keys = append(keys, outbound.DeduplicationKey{Strategy: dedupStrategyLogical, Key: scopedKey(config, valid.LogicalEventID)})
	}
	if valid.DedupStrategy == dedupStrategySemantic {
		raw := dto.RawEvent{
			EventName:   valid.EventName,
			EventTime:   valid.EventTime,
			SiteCode:    valid.SiteCode,
			SessionID:   valid.SessionID,
			TabID:       valid.TabID,
			Path:        valid.Path,
			URL:         valid.URL,
			AnonymousID: valid.AnonymousID,
		}
		semantic := semanticDedupCandidate(raw, config, uc.semanticRules)
		if !semantic.Empty {
			keys = append(keys, semantic.Key)
		}
	}
	keys = append(keys, itemSpecificDedupKeys(valid)...)
	for _, key := range keys {
		if err := uc.deduplicator.Mark(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// rejectionReason traduce errores de validacion a motivos persistibles.
//
// Recibe errores de dominio o aplicacion y devuelve un codigo estable para
// analytics_rejected_events.reason.
func rejectionReason(err error) string {
	switch {
	case errors.Is(err, site.ErrInvalidConfig):
		return rejection.ReasonInvalidSiteConfig
	case errors.Is(err, site.ErrSiteInactive):
		return rejection.ReasonSiteInactive
	case errors.Is(err, site.ErrTrackingDisabled):
		return rejection.ReasonTrackingDisabled
	case errors.Is(err, site.ErrTokenVersion):
		return rejection.ReasonTokenVersion
	case errors.Is(err, site.ErrDomainNotAllowed):
		return rejection.ReasonDomainNotAllowed
	case errors.Is(err, ErrSiteNotAvailable):
		return rejection.ReasonSiteUnresolved
	default:
		return rejection.ReasonInvalidSiteConfig
	}
}

// rejectionSeverity traduce errores de validacion a severidades.
//
// Recibe errores de dominio o aplicacion y devuelve una severidad estable para
// auditoria. Los dominios no permitidos se marcan como sospechosos.
func rejectionSeverity(err error) string {
	if errors.Is(err, site.ErrDomainNotAllowed) {
		return rejection.SeveritySuspicious
	}
	return rejection.SeverityWarning
}
