package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/worker/internal/domain/subject"
)

type suggestionRepositoryFake struct {
	availability        subject.ProjectionAvailability
	results             []subject.Suggestion
	contextResults      []subject.Suggestion
	contextRegistered   bool
	contextAvailability *subject.ProjectionAvailability
	contextStatus       string
}

func (fake *suggestionRepositoryFake) ContextResolutionStatus(context.Context, string, int) (string, error) {
	if fake.contextStatus != "" {
		return fake.contextStatus, nil
	}
	return "unregistered", nil
}

func (fake *suggestionRepositoryFake) ContextSuggestions(context.Context, string, string, int, int, int, int) ([]subject.Suggestion, bool, error) {
	return fake.contextResults, fake.contextRegistered, nil
}

func (fake *suggestionRepositoryFake) ProjectionAvailability(_ context.Context, _ int, names ...string) (subject.ProjectionAvailability, error) {
	if len(names) > 0 && fake.contextAvailability != nil {
		return *fake.contextAvailability, nil
	}
	return fake.availability, nil
}

func TestQuerySuggestionsUsesExactContextThenGlobalFallback(t *testing.T) {
	now := time.Now().UTC()
	available := subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: now}
	repository := &suggestionRepositoryFake{availability: available, results: []subject.Suggestion{{ExternalSubjectID: "global"}}, contextResults: []subject.Suggestion{{ExternalSubjectID: "exact", SourceLevel: "context"}}, contextRegistered: true}
	useCase := NewQuerySuggestions(repository, 1, 90, 30, 48*time.Hour, true)
	response, err := useCase.Execute(context.Background(), SuggestionQuery{TenantID: "tenant", Limit: 1, ContextKey: "search", ContextVersion: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	if response.Results[0].ExternalSubjectID != "exact" || response.Context["source_level"] != "context" {
		t.Fatalf("response=%#v", response)
	}
	repository.contextResults = nil
	response, err = useCase.Execute(context.Background(), SuggestionQuery{TenantID: "tenant", Limit: 10, ContextKey: "search", ContextVersion: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	if response.Results[0].ExternalSubjectID != "global" || response.Context["fallback_applied"] != true {
		t.Fatalf("fallback=%#v", response)
	}
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

func TestQuerySuggestionsReportsContextProjectionFreshness(t *testing.T) {
	now := time.Now().UTC()
	old := subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: now.Add(-49 * time.Hour)}
	repo := &suggestionRepositoryFake{availability: subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: now}, contextAvailability: &old, contextRegistered: true, contextResults: []subject.Suggestion{{ExternalSubjectID: "1"}}}
	query := SuggestionQuery{TenantID: "t", Limit: 10, ContextKey: "search", ContextVersion: 1}
	useCase := NewQuerySuggestions(repo, 1, 90, 30, 48*time.Hour, false)
	if _, err := useCase.Execute(context.Background(), query, now); !errors.Is(err, ErrProjectionStale) {
		t.Fatalf("err=%v", err)
	}
	useCase.serveStale = true
	response, err := useCase.Execute(context.Background(), query, now)
	if err != nil || !response.IsStale || !response.ComputedAt.Equal(old.ComputedAt) {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}

func TestQuerySuggestionsCompletesPartialContextWithoutDuplicates(t *testing.T) {
	now := time.Now().UTC()
	repo := &suggestionRepositoryFake{availability: subject.ProjectionAvailability{Available: true, ProjectionVersion: 1, ComputedAt: now}, contextRegistered: true, contextResults: []subject.Suggestion{{ExternalSubjectID: "exact", SourceLevel: "context", AssociationCount: 2}}, results: []subject.Suggestion{{ExternalSubjectID: "exact", SourceLevel: "global", AssociationCount: 99}, {ExternalSubjectID: "other", SourceLevel: "global", AssociationCount: 3}}}
	useCase := NewQuerySuggestions(repo, 1, 90, 30, 48*time.Hour, true)
	response, err := useCase.Execute(context.Background(), SuggestionQuery{TenantID: "t", Limit: 2, ContextKey: "search", ContextVersion: 1}, now)
	if err != nil || len(response.Results) != 2 || response.Results[0].AssociationCount != 2 || response.Results[1].ExternalSubjectID != "other" || response.Context["source_level"] != "mixed" {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}
