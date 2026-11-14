package event

import (
	"errors"
	"testing"
	"time"
)

func TestValidateInternalEventProfiles(t *testing.T) {
	t.Parallel()

	valid := validInternalEvent()
	nonUTC := valid.Timestamp.In(time.FixedZone("UTC-3", -3*60*60))
	tests := []struct {
		name    string
		mutate  func(*InternalEvent)
		wantErr error
	}{
		{name: "asociación válida"},
		{name: "evento desconocido", mutate: func(input *InternalEvent) { input.EventName = "customer_selected" }, wantErr: ErrUnsupportedInternalEvent},
		{name: "tipo no soportado", mutate: func(input *InternalEvent) { input.Subjects[0].SubjectType = "account" }, wantErr: ErrUnsupportedSubjectType},
		{name: "sin idempotencia", mutate: func(input *InternalEvent) { input.IdempotencyKey = "" }, wantErr: ErrInvalidInternalEvent},
		{name: "timestamp no UTC", mutate: func(input *InternalEvent) { input.Timestamp = nonUTC }, wantErr: ErrInvalidInternalEvent},
		{name: "sin sujetos", mutate: func(input *InternalEvent) { input.Subjects = nil }, wantErr: ErrInvalidInternalEvent},
		{name: "lifecycle en asociación", mutate: func(input *InternalEvent) {
			input.Lifecycle = &LifecycleChange{SourceUpdatedAt: valid.Timestamp, SourceMutationID: "m1"}
		}, wantErr: ErrInvalidInternalEvent},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validInternalEvent()
			if test.mutate != nil {
				test.mutate(&input)
			}
			if err := ValidateInternalEvent(input); !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidateInternalEvent() error = %v, esperado %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateInternalEventLifecycleAndMerge(t *testing.T) {
	t.Parallel()

	input := validInternalEvent()
	input.EventName = "subject_merged"
	input.Lifecycle = &LifecycleChange{
		SourceUpdatedAt:     input.Timestamp.Add(-time.Second),
		SourceMutationID:    "mutation-1",
		MergedIntoSubjectID: "customer-2",
	}
	if err := ValidateInternalEvent(input); err != nil {
		t.Fatalf("ValidateInternalEvent() error = %v", err)
	}

	input.Lifecycle.MergedIntoSubjectID = ""
	if !errors.Is(ValidateInternalEvent(input), ErrInvalidInternalEvent) {
		t.Fatal("subject_merged sin destino debería ser inválido")
	}

	input.EventName = "subject_deactivated"
	input.Lifecycle.MergedIntoSubjectID = "customer-2"
	if !errors.Is(ValidateInternalEvent(input), ErrInvalidInternalEvent) {
		t.Fatal("subject_deactivated con destino debería ser inválido")
	}
}

func validInternalEvent() InternalEvent {
	return InternalEvent{
		EventID:        "event-1",
		IdempotencyKey: "operation-1",
		EventName:      "subject_associated",
		EventVersion:   1,
		Timestamp:      time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		SiteCode:       "site-1",
		Subjects: []SubjectReference{{
			SubjectType: "customer",
			SubjectID:   "customer-1",
			Relation:    "attendee",
		}},
	}
}
