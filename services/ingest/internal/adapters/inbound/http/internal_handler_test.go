package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goanalytics/services/ingest/internal/application/dto"
)

const validInternalBody = `{
  "event_id":"event-1",
  "idempotency_key":"operation-1",
  "event_name":"subject_associated",
  "event_version":1,
  "timestamp":"2026-08-10T12:00:00Z",
  "site_code":"site-1",
  "properties":{"subjects":[{"subject_type":"customer","subject_id":"customer-1","relation":"attendee"}]},
  "context":{}
}`

func TestInternalIngestHandlerAcceptsDedicatedBearer(t *testing.T) {
	t.Parallel()

	ingester := &fakeInternalIngester{response: dto.IngestResponse{Accepted: 1, EventIDs: []string{"event-1"}}}
	handler := NewInternalIngestHandler(ingester, nil, 65536, "internal-secret")
	request := httptest.NewRequest(http.MethodPost, "/v1/internal/events", strings.NewReader(validInternalBody))
	request.Header.Set("Authorization", "Bearer internal-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(ingester.request.Events) != 1 || ingester.request.Events[0].SiteCode != "site-1" {
		t.Fatalf("request = %#v", ingester.request)
	}
}

func TestInternalIngestHandlerRejectsInvalidBearer(t *testing.T) {
	t.Parallel()

	handler := NewInternalIngestHandler(&fakeInternalIngester{}, nil, 65536, "internal-secret")
	for _, authorization := range []string{"", "Bearer public-token", "Basic internal-secret"} {
		request := httptest.NewRequest(http.MethodPost, "/v1/internal/events", strings.NewReader(validInternalBody))
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized || internalErrorCode(t, response) != "invalid_token" {
			t.Fatalf("authorization %q produjo status %d body %s", authorization, response.Code, response.Body.String())
		}
	}
}

func TestInternalIngestHandlerUsesStrictServerSchema(t *testing.T) {
	t.Parallel()

	handler := NewInternalIngestHandler(&fakeInternalIngester{}, nil, 65536, "internal-secret")
	body := strings.Replace(validInternalBody, `"context":{}`, `"origin":"https://cliente.example","context":{}`, 1)
	request := httptest.NewRequest(http.MethodPost, "/v1/internal/events", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer internal-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || internalErrorCode(t, response) != "invalid_payload" {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestInternalIngestHandlerEnforcesPayloadLimit(t *testing.T) {
	t.Parallel()

	handler := NewInternalIngestHandler(&fakeInternalIngester{}, nil, 20, "internal-secret")
	request := httptest.NewRequest(http.MethodPost, "/v1/internal/events", strings.NewReader(validInternalBody))
	request.Header.Set("Authorization", "Bearer internal-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || internalErrorCode(t, response) != "payload_too_large" {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestRouterHidesInternalEndpointWhenNotRegistered(t *testing.T) {
	t.Parallel()

	publicHandler := NewIngestHandler(fakeIngester{}, nil, 0, false)
	router := NewRouter(publicHandler, NewHealthHandler(), NewHealthHandler(), nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/internal/events", strings.NewReader(validInternalBody))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, esperado 404", response.Code)
	}
}

func internalErrorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("respuesta JSON inválida: %v", err)
	}
	return payload["error"]
}

type fakeInternalIngester struct {
	request  dto.InternalIngestRequest
	response dto.IngestResponse
	err      error
}

func (fake *fakeInternalIngester) IngestInternal(ctx context.Context, request dto.InternalIngestRequest) (dto.IngestResponse, error) {
	_ = ctx
	fake.request = request
	return fake.response, fake.err
}
