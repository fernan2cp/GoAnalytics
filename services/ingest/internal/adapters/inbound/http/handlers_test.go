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

func TestIngestHandlerReturnsAcceptedJSON(t *testing.T) {
	handler := NewIngestHandler(fakeIngester{
		response: dto.IngestResponse{
			Accepted: 1,
			Rejected: 1,
			EventIDs: []string{"evt_1"},
		},
	}, nil, 0, false)
	request := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_id":"evt_1"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", response.Code)
	}
	var payload dto.IngestResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if payload.Accepted != 1 || payload.Rejected != 1 || len(payload.EventIDs) != 1 {
		t.Fatalf("payload = %#v, want accepted=1 rejected=1 event_ids", payload)
	}
}

type fakeIngester struct {
	response dto.IngestResponse
	err      error
}

func (fake fakeIngester) Ingest(ctx context.Context, request dto.IngestRequest) (dto.IngestResponse, error) {
	_ = ctx
	_ = request
	return fake.response, fake.err
}
