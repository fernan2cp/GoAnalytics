package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"time"

	"goanalytics/services/ingest/internal/application/dto"
	"goanalytics/services/ingest/internal/domain/event"
)

// internalBatchPayload representa la envoltura opcional del contrato interno.
type internalBatchPayload struct {
	Events []internalEventPayload `json:"events"`
}

// internalEventPayload representa un evento server-to-server estricto.
type internalEventPayload struct {
	EventID        string                    `json:"event_id"`
	IdempotencyKey string                    `json:"idempotency_key"`
	EventName      string                    `json:"event_name"`
	EventVersion   int                       `json:"event_version"`
	Timestamp      time.Time                 `json:"timestamp"`
	SiteCode       string                    `json:"site_code"`
	Properties     internalPropertiesPayload `json:"properties"`
	Context        map[string]any            `json:"context"`
}

type internalPropertiesPayload struct {
	Subjects  []internalSubjectPayload  `json:"subjects"`
	Operation *internalOperationPayload `json:"operation,omitempty"`
	Lifecycle *internalLifecyclePayload `json:"lifecycle,omitempty"`
}

type internalSubjectPayload struct {
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	Relation    string `json:"relation,omitempty"`
}

type internalOperationPayload struct {
	OperationType string `json:"operation_type"`
	OperationID   string `json:"operation_id"`
}

type internalLifecyclePayload struct {
	SourceUpdatedAt     time.Time  `json:"source_updated_at"`
	SourceMutationID    string     `json:"source_mutation_id"`
	EffectiveAt         *time.Time `json:"effective_at,omitempty"`
	MergedIntoSubjectID string     `json:"merged_into_subject_id,omitempty"`
}

// decodeInternalIngestRequest decodifica objeto único o batch interno.
//
// Usa un esquema estricto y devuelve error ante campos desconocidos, JSON
// adicional o una raíz distinta de objeto. No ejecuta autenticación.
func decodeInternalIngestRequest(request *nethttp.Request) (dto.InternalIngestRequest, error) {
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return dto.InternalIngestRequest{}, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return dto.InternalIngestRequest{}, fmt.Errorf("objeto JSON requerido")
	}

	var payloads []internalEventPayload
	if _, batch := root["events"]; batch {
		var payload internalBatchPayload
		if err := decodeStrictJSON(data, &payload); err != nil {
			return dto.InternalIngestRequest{}, err
		}
		payloads = payload.Events
	} else {
		var payload internalEventPayload
		if err := decodeStrictJSON(data, &payload); err != nil {
			return dto.InternalIngestRequest{}, err
		}
		payloads = []internalEventPayload{payload}
	}

	events := make([]event.InternalEvent, 0, len(payloads))
	for _, payload := range payloads {
		events = append(events, payload.toDomain())
	}
	return dto.InternalIngestRequest{Events: events}, nil
}

// decodeStrictJSON decodifica un único valor y prohíbe campos desconocidos.
func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("contenido JSON adicional")
	}
	return nil
}

// toDomain transforma el payload HTTP a un evento de dominio.
func (payload internalEventPayload) toDomain() event.InternalEvent {
	subjects := make([]event.SubjectReference, 0, len(payload.Properties.Subjects))
	for _, subject := range payload.Properties.Subjects {
		subjects = append(subjects, event.SubjectReference{
			SubjectType: subject.SubjectType,
			SubjectID:   subject.SubjectID,
			Relation:    subject.Relation,
		})
	}
	var operation *event.OperationReference
	if payload.Properties.Operation != nil {
		operation = &event.OperationReference{
			OperationType: payload.Properties.Operation.OperationType,
			OperationID:   payload.Properties.Operation.OperationID,
		}
	}
	var lifecycle *event.LifecycleChange
	if payload.Properties.Lifecycle != nil {
		lifecycle = &event.LifecycleChange{
			SourceUpdatedAt:     payload.Properties.Lifecycle.SourceUpdatedAt,
			SourceMutationID:    payload.Properties.Lifecycle.SourceMutationID,
			EffectiveAt:         payload.Properties.Lifecycle.EffectiveAt,
			MergedIntoSubjectID: payload.Properties.Lifecycle.MergedIntoSubjectID,
		}
	}
	return event.InternalEvent{
		EventID:        payload.EventID,
		IdempotencyKey: payload.IdempotencyKey,
		EventName:      payload.EventName,
		EventVersion:   payload.EventVersion,
		Timestamp:      payload.Timestamp,
		SiteCode:       payload.SiteCode,
		Subjects:       subjects,
		Operation:      operation,
		Lifecycle:      lifecycle,
		Context:        payload.Context,
	}
}
