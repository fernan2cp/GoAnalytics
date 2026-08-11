package usecases

import (
	"testing"
	"time"

	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/subject"
)

func TestNormalizeSubjectDetailsPreservesFirstPositionAndCalendarSnapshot(t *testing.T) {
	t.Parallel()

	valid := normalizationEvent()
	valid.Properties = map[string]any{
		"subjects": []any{
			map[string]any{"subject_type": "customer", "subject_id": "customer-1", "relation": "attendee"},
			map[string]any{"subject_type": "customer", "subject_id": "customer-1", "relation": "attendee"},
			map[string]any{"subject_type": "customer", "subject_id": "customer-2", "relation": "guest"},
		},
		"operation": map[string]any{"operation_type": "appointment", "operation_id": "845"},
	}
	valid.Context = map[string]any{"context_key": "appointments.customer_picker", "context_version": float64(1)}

	details := normalizeSubjectDetails(valid)
	if len(details) != 2 {
		t.Fatalf("detalles = %d, esperado 2", len(details))
	}
	if details[0].SubjectOrdinal != 0 || details[1].SubjectOrdinal != 2 {
		t.Fatalf("ordinales = %d/%d", details[0].SubjectOrdinal, details[1].SubjectOrdinal)
	}
	if details[0].NormalizationStatus != subject.NormalizationNormalized || details[0].LocalDate == nil || details[0].LocalDate.Format("2006-01-02") != "2026-08-09" {
		t.Fatalf("normalización = %#v", details[0])
	}
	if details[0].ContextVersion == nil || *details[0].ContextVersion != 1 || details[0].ContextKey != "appointments.customer_picker" {
		t.Fatalf("contexto = %#v", details[0])
	}
}

func TestNormalizeSubjectDetailsKeepsSemanticFailuresAuditable(t *testing.T) {
	t.Parallel()

	valid := normalizationEvent()
	valid.Timezone = ""
	valid.Properties = map[string]any{"subjects": []any{
		"not-an-object",
		map[string]any{"subject_type": "account", "subject_id": "account-1"},
		map[string]any{"subject_type": "customer", "subject_id": "customer-1"},
	}}

	details := normalizeSubjectDetails(valid)
	if len(details) != 3 {
		t.Fatalf("detalles = %d", len(details))
	}
	if details[0].NormalizationStatus != subject.NormalizationInvalidSubject {
		t.Fatalf("detalle inválido = %#v", details[0])
	}
	if details[1].NormalizationStatus != subject.NormalizationUnsupportedType {
		t.Fatalf("tipo no soportado = %#v", details[1])
	}
	if details[2].NormalizationStatus != subject.NormalizationTimezoneUnresolved || details[2].LocalDate != nil {
		t.Fatalf("timezone no resuelta = %#v", details[2])
	}
}

func TestNormalizeSubjectDetailsCreatesActiveOnlyForServerAssociation(t *testing.T) {
	t.Parallel()

	valid := normalizationEvent()
	valid.EventSource = event.SourceServer
	valid.EventName = "subject_associated"
	detail := normalizeSubjectDetails(valid)[0]
	if detail.InitialLifecycleStatus != subject.LifecycleActive {
		t.Fatalf("estado = %q, esperado active", detail.InitialLifecycleStatus)
	}

	valid.EventSource = event.SourceWeb
	detail = normalizeSubjectDetails(valid)[0]
	if detail.InitialLifecycleStatus != subject.LifecycleUnknown {
		t.Fatalf("estado web = %q, esperado unknown", detail.InitialLifecycleStatus)
	}
}

func TestNormalizeSubjectDetailsHandlesAbsentAndMalformedCollection(t *testing.T) {
	t.Parallel()

	valid := normalizationEvent()
	valid.Properties = map[string]any{}
	if details := normalizeSubjectDetails(valid); len(details) != 0 {
		t.Fatalf("detalles ausentes = %#v", details)
	}
	valid.Properties["subjects"] = map[string]any{}
	details := normalizeSubjectDetails(valid)
	if len(details) != 1 || details[0].NormalizationReason != "subjects_not_array" {
		t.Fatalf("colección inválida = %#v", details)
	}
}

func normalizationEvent() event.ValidatedEvent {
	return event.ValidatedEvent{
		EventID:     "event-1",
		EventSource: event.SourceWeb,
		EventName:   "subject_selected",
		EventTime:   time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC),
		Timezone:    "America/Argentina/Buenos_Aires",
		Properties: map[string]any{"subjects": []any{
			map[string]any{"subject_type": "customer", "subject_id": "customer-1", "relation": "selected"},
		}},
	}
}
