package http

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeInternalIngestRequestAcceptsBatchLifecycle(t *testing.T) {
	t.Parallel()

	body := `{"events":[{
      "event_id":"event-2",
      "idempotency_key":"mutation-2",
      "event_name":"subject_merged",
      "event_version":1,
      "timestamp":"2026-08-10T12:00:01Z",
      "site_code":"site-1",
      "properties":{
        "subjects":[{"subject_type":"customer","subject_id":"customer-1","relation":"lifecycle"}],
        "lifecycle":{"source_updated_at":"2026-08-10T12:00:00Z","source_mutation_id":"mutation-2","merged_into_subject_id":"customer-2"}
      },
      "context":{}
    }]}`
	request := httptest.NewRequest("POST", "/v1/internal/events", strings.NewReader(body))

	decoded, err := decodeInternalIngestRequest(request)
	if err != nil {
		t.Fatalf("decodeInternalIngestRequest() error = %v", err)
	}
	if len(decoded.Events) != 1 || decoded.Events[0].Lifecycle == nil {
		t.Fatalf("eventos = %#v", decoded.Events)
	}
	if decoded.Events[0].Lifecycle.MergedIntoSubjectID != "customer-2" {
		t.Fatalf("destino merge = %q", decoded.Events[0].Lifecycle.MergedIntoSubjectID)
	}
}

func TestDecodeInternalIngestRequestRejectsUnknownNestedField(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validInternalBody, `"relation":"attendee"`, `"relation":"attendee","email":"secret@example.com"`, 1)
	request := httptest.NewRequest("POST", "/v1/internal/events", strings.NewReader(body))
	if _, err := decodeInternalIngestRequest(request); err == nil {
		t.Fatal("se esperaba rechazo de campo anidado desconocido")
	}
}
