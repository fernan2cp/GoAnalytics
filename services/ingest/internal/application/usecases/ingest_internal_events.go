package usecases

import (
	"context"
	"fmt"
	"time"

	"goanalytics/services/ingest/internal/application/dto"
	"goanalytics/services/ingest/internal/application/ports/outbound"
	"goanalytics/services/ingest/internal/domain/event"
)

// IngestInternalEventsUseCase valida y publica eventos server-to-server.
//
// Usa el mismo puerto EventPublisher que la ingesta pública, pero construye
// eventos con source server y sin inventar identidad de sesión o navegación.
type IngestInternalEventsUseCase struct {
	publisher         outbound.EventPublisher
	clock             outbound.Clock
	maxEventsPerBatch int
}

// NewIngestInternalEventsUseCase crea el caso de uso de ingesta interna.
//
// Recibe publisher, reloj y máximo de eventos. Devuelve una instancia lista
// para ejecutarse; dependencias ausentes se informan al llamar IngestInternal.
func NewIngestInternalEventsUseCase(publisher outbound.EventPublisher, clock outbound.Clock, maxEventsPerBatch int) *IngestInternalEventsUseCase {
	if maxEventsPerBatch <= 0 {
		maxEventsPerBatch = DefaultMaxEventsPerBatch
	}
	return &IngestInternalEventsUseCase{publisher: publisher, clock: clock, maxEventsPerBatch: maxEventsPerBatch}
}

// IngestInternal valida un batch interno y lo publica en el stream compartido.
//
// Devuelve ErrInvalidBatch o ErrInvalidPayload sin publicar parcialmente si
// cualquier perfil es inválido. Devuelve ErrPublishFailed ante fallas del
// stream y una respuesta con todos los IDs cuando el batch queda aceptado.
func (uc *IngestInternalEventsUseCase) IngestInternal(ctx context.Context, request dto.InternalIngestRequest) (dto.IngestResponse, error) {
	if uc == nil || uc.publisher == nil || uc.clock == nil {
		return dto.IngestResponse{}, ErrDependencyMissing
	}
	if len(request.Events) == 0 || len(request.Events) > uc.maxEventsPerBatch {
		return dto.IngestResponse{}, ErrInvalidBatch
	}

	receivedAt := uc.clock.Now()
	rawEvents := make([]event.RawEvent, 0, len(request.Events))
	eventIDs := make([]string, 0, len(request.Events))
	for _, item := range request.Events {
		if err := event.ValidateInternalEvent(item); err != nil {
			return dto.IngestResponse{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
		}
		rawEvents = append(rawEvents, mapInternalRawEvent(item, receivedAt))
		eventIDs = append(eventIDs, item.EventID)
	}
	if err := uc.publisher.Publish(ctx, rawEvents); err != nil {
		return dto.IngestResponse{}, fmt.Errorf("%w: %v", ErrPublishFailed, err)
	}
	return dto.IngestResponse{Accepted: len(rawEvents), EventIDs: eventIDs}, nil
}

// mapInternalRawEvent transforma el contrato tipado al payload extensible.
func mapInternalRawEvent(input event.InternalEvent, receivedAt time.Time) event.RawEvent {
	properties := map[string]any{"subjects": uniqueSubjects(input.Subjects)}
	if input.Operation != nil {
		properties["operation"] = map[string]any{
			"operation_type": input.Operation.OperationType,
			"operation_id":   input.Operation.OperationID,
		}
	}
	if input.Lifecycle != nil {
		lifecycle := map[string]any{
			"source_updated_at":  input.Lifecycle.SourceUpdatedAt.Format(time.RFC3339Nano),
			"source_mutation_id": input.Lifecycle.SourceMutationID,
		}
		if input.Lifecycle.EffectiveAt != nil {
			lifecycle["effective_at"] = input.Lifecycle.EffectiveAt.Format(time.RFC3339Nano)
		}
		if input.Lifecycle.MergedIntoSubjectID != "" {
			lifecycle["merged_into_subject_id"] = input.Lifecycle.MergedIntoSubjectID
		}
		properties["lifecycle"] = lifecycle
	}

	return event.RawEvent{
		EventID:        input.EventID,
		IdempotencyKey: input.IdempotencyKey,
		EventSource:    event.SourceServer,
		SiteCode:       input.SiteCode,
		Environment:    "production",
		EventName:      input.EventName,
		EventVersion:   input.EventVersion,
		EventTime:      input.Timestamp,
		ReceivedAt:     receivedAt,
		SDKName:        "goanalytics-server",
		SDKVersion:     "1.0.0",
		Properties:     properties,
		Context:        event.NormalizeMap(input.Context),
	}
}

// uniqueSubjects elimina duplicados exactos conservando la primera posición.
func uniqueSubjects(subjects []event.SubjectReference) []map[string]any {
	result := make([]map[string]any, 0, len(subjects))
	seen := make(map[string]struct{}, len(subjects))
	for _, subject := range subjects {
		key := subject.SubjectType + "\x00" + subject.SubjectID + "\x00" + subject.Relation
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, map[string]any{
			"subject_type": subject.SubjectType,
			"subject_id":   subject.SubjectID,
			"relation":     subject.Relation,
		})
	}
	return result
}
