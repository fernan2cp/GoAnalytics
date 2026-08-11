package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/ingest/internal/application/dto"
	"goanalytics/services/ingest/internal/domain/event"
)

func TestIngestInternalPublishesServerEventWithoutNavigation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 13, 0, 0, 0, time.UTC)
	publisher := &fakePublisher{}
	useCase := NewIngestInternalEventsUseCase(publisher, fakeClock{now: now}, 50)
	input := validInternalRequest()
	input.Events[0].Subjects = append(input.Events[0].Subjects, input.Events[0].Subjects[0])

	response, err := useCase.IngestInternal(context.Background(), input)
	if err != nil {
		t.Fatalf("IngestInternal() error = %v", err)
	}
	if response.Accepted != 1 || len(response.EventIDs) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("eventos publicados = %d", len(publisher.events))
	}
	published := publisher.events[0]
	if published.EventSource != event.SourceServer || published.SiteCode != "site-1" || published.ReceivedAt != now {
		t.Fatalf("evento publicado = %#v", published)
	}
	if published.SessionID != "" || published.Origin != "" || published.URL != "" || published.Path != "" {
		t.Fatal("el perfil server no debe inventar navegación ni sesión")
	}
	subjects, ok := published.Properties["subjects"].([]map[string]any)
	if !ok || len(subjects) != 1 {
		t.Fatalf("subjects = %#v, esperado duplicado eliminado", published.Properties["subjects"])
	}
}

func TestIngestInternalRejectsWholeInvalidBatch(t *testing.T) {
	t.Parallel()

	publisher := &fakePublisher{}
	useCase := NewIngestInternalEventsUseCase(publisher, fakeClock{now: time.Now()}, 50)
	input := validInternalRequest()
	invalid := input.Events[0]
	invalid.Subjects[0].SubjectType = "account"
	input.Events = append(input.Events, invalid)

	_, err := useCase.IngestInternal(context.Background(), input)
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("IngestInternal() error = %v", err)
	}
	if len(publisher.events) != 0 {
		t.Fatal("un batch inválido no debe publicarse parcialmente")
	}
}

func TestIngestInternalEnforcesBatchLimitAndPublishErrors(t *testing.T) {
	t.Parallel()

	input := validInternalRequest()
	input.Events = append(input.Events, input.Events[0])
	useCase := NewIngestInternalEventsUseCase(&fakePublisher{}, fakeClock{now: time.Now()}, 1)
	if _, err := useCase.IngestInternal(context.Background(), input); !errors.Is(err, ErrInvalidBatch) {
		t.Fatalf("error = %v, esperado ErrInvalidBatch", err)
	}

	publisher := &fakePublisher{err: errors.New("redis caído")}
	useCase = NewIngestInternalEventsUseCase(publisher, fakeClock{now: time.Now()}, 50)
	if _, err := useCase.IngestInternal(context.Background(), validInternalRequest()); !errors.Is(err, ErrPublishFailed) {
		t.Fatalf("error = %v, esperado ErrPublishFailed", err)
	}
}

func validInternalRequest() dto.InternalIngestRequest {
	return dto.InternalIngestRequest{Events: []event.InternalEvent{{
		EventID:        "event-1",
		IdempotencyKey: "operation-1",
		EventName:      "subject_associated",
		EventVersion:   1,
		Timestamp:      time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		SiteCode:       "site-1",
		Subjects: []event.SubjectReference{{
			SubjectType: "customer",
			SubjectID:   "customer-1",
			Relation:    "attendee",
		}},
	}}}
}
