package postgres

import (
	"context"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/usecases"
	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/subject"
)

func TestSubjectMVPEndToEndPersistsProjectsAndSuggestsContext(t *testing.T) {
	events, pool := newIntegrationEventRepository(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, projectionIntegrationSchemaSQL+mvpContextIntegrationSchema); err != nil {
		t.Fatal(err)
	}
	value := integrationValidatedEvent("e2e-association")
	value.EventName = "subject_associated"
	value.EventSource = "server"
	value.IdempotencyKey = "sale:8:customer:42:buyer"
	localDate := time.Now().UTC().Truncate(24 * time.Hour)
	definitionID := int64(1)
	revisionID := int64(1)
	version := 1
	value.SubjectDetails = []subject.EventDetail{{SubjectOrdinal: 0, SubjectType: "customer", ExternalSubjectID: "42", Relation: "buyer", ContextKey: "sales.customer_association", ContextVersion: &version, ClaimedContextFamily: "transaction", ResolvedContextFamily: "transaction", ContextDefinitionID: &definitionID, ContextRegistryRevisionID: &revisionID, ContextResolutionStatus: subject.ContextRegistered, TimezoneName: "UTC", LocalDate: &localDate, NormalizationStatus: subject.NormalizationNormalized, InitialLifecycleStatus: subject.LifecycleActive}}
	result := events.SaveBatch(ctx, []event.ValidatedEvent{value})
	if len(result) != 1 || !result[0].IsIdempotentSuccess() {
		t.Fatalf("persistencia=%#v", result)
	}
	projections, _ := NewProjectionRepository(pool)
	global, _ := usecases.NewProjectGlobal(projections, nil, 1, 100)
	contextual, _ := usecases.NewProjectContext(projections, 1, 100)
	if err := global.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := contextual.Run(ctx); err != nil {
		t.Fatal(err)
	}
	queries, _ := NewSuggestionRepository(pool)
	useCase := usecases.NewQuerySuggestions(queries, 1, 90, 30, 48*time.Hour, true)
	response, err := useCase.Execute(ctx, usecases.SuggestionQuery{TenantID: "tenant-1", Limit: 10, ContextKey: "sales.customer_association", ContextVersion: 1}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].ExternalSubjectID != "42" || response.Context["source_level"] != "context" {
		t.Fatalf("respuesta=%#v", response)
	}
}

const mvpContextIntegrationSchema = `
CREATE TABLE analytics_context_registry_revisions(id BIGINT PRIMARY KEY,validation_status TEXT,activated_at TIMESTAMPTZ);INSERT INTO analytics_context_registry_revisions VALUES(1,'valid',now());
CREATE TABLE analytics_context_definitions(id BIGINT PRIMARY KEY,context_key TEXT,context_version INTEGER);INSERT INTO analytics_context_definitions VALUES(1,'sales.customer_association',1);
CREATE TABLE analytics_context_registry_entries(registry_revision_id BIGINT,context_definition_id BIGINT,status TEXT);INSERT INTO analytics_context_registry_entries VALUES(1,1,'active');
CREATE TABLE analytics_subject_context_daily(tenant_id TEXT,analytics_subject_id BIGINT,context_definition_id BIGINT,local_date DATE,timezone_name TEXT,impression_count BIGINT DEFAULT 0,view_count BIGINT DEFAULT 0,selection_count BIGINT DEFAULT 0,association_count BIGINT DEFAULT 0,last_association_at TIMESTAMPTZ,projection_version INTEGER,computed_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT now(),updated_at TIMESTAMPTZ DEFAULT now(),PRIMARY KEY(tenant_id,analytics_subject_id,context_definition_id,local_date,timezone_name,projection_version));`
