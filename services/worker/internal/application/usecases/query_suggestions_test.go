package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/worker/internal/domain/subject"
)

type suggestionRepositoryFake struct {
	availability subject.ProjectionAvailability
	results      []subject.Suggestion
}

func (fake *suggestionRepositoryFake) ProjectionAvailability(context.Context, int) (subject.ProjectionAvailability, error) {
	return fake.availability, nil
}
func (fake *suggestionRepositoryFake) GlobalSuggestions(context.Context, string, int, int, int) ([]subject.Suggestion, error) {
	return fake.results, nil
}

func TestQuerySuggestionsUnavailableAndStale(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	useCase := NewQuerySuggestions(&suggestionRepositoryFake{}, 1, 90, 30, 48*time.Hour, true)
	if _, err := useCase.Execute(context.Background(), SuggestionQuery{TenantID: "tenant", Limit: 10}, now); !errors.Is(err, ErrProjectionUnavailable) {
		t.Fatalf("err=%v", err)
	}
	useCase = NewQuerySuggestions(&suggestionRepositoryFake{availability: subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: now.Add(-49 * time.Hour)}}, 1, 90, 30, 48*time.Hour, false)
	if _, err := useCase.Execute(context.Background(), SuggestionQuery{TenantID: "tenant", Limit: 10}, now); !errors.Is(err, ErrProjectionStale) {
		t.Fatalf("err=%v", err)
	}
}

func TestQuerySuggestionsServesStaleAndEmpty(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	useCase := NewQuerySuggestions(&suggestionRepositoryFake{availability: subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: now.Add(-49 * time.Hour)}}, 1, 90, 30, 48*time.Hour, true)
	response, err := useCase.Execute(context.Background(), SuggestionQuery{TenantID: "tenant", Limit: 10}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsStale || response.Results == nil || len(response.Results) != 0 {
		t.Fatalf("response=%#v", response)
	}
}
