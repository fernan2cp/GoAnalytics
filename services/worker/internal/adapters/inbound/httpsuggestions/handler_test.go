package httpsuggestions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/usecases"
	"goanalytics/services/worker/internal/domain/subject"
)

type repositoryFake struct{}

func (repositoryFake) ProjectionAvailability(context.Context, int) (subject.ProjectionAvailability, error) {
	return subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: time.Now()}, nil
}
func (repositoryFake) GlobalSuggestions(context.Context, string, int, int, int) ([]subject.Suggestion, error) {
	return []subject.Suggestion{}, nil
}

func TestHandlerAuthenticatesAndValidatesStrictQuery(t *testing.T) {
	query := usecases.NewQuerySuggestions(repositoryFake{}, 1, 90, 30, 48*time.Hour, true)
	handler := NewHandler(true, "secret", 10, 100, query)
	for _, test := range []struct {
		url, token string
		status     int
	}{{"/v1/internal/subjects/suggestions?tenant_id=t&subject_type=customer", "", 401}, {"/v1/internal/subjects/suggestions?tenant_id=t&subject_type=customer&unknown=x", "Bearer secret", 400}, {"/v1/internal/subjects/suggestions?tenant_id=t&subject_type=customer&context_key=x", "Bearer secret", 400}, {"/v1/internal/subjects/suggestions?tenant_id=t&subject_type=customer&limit=101", "Bearer secret", 400}, {"/v1/internal/subjects/suggestions?tenant_id=t&subject_type=customer", "Bearer secret", 200}} {
		request := httptest.NewRequest(http.MethodGet, test.url, nil)
		request.Header.Set("Authorization", test.token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("url=%s status=%d body=%s", test.url, response.Code, response.Body.String())
		}
	}
}
