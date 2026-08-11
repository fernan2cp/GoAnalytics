package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/subject"
)

func TestEventRepositoryIntegrationClassifiesConflictsPerEvent(t *testing.T) {
	repository, pool := newIntegrationEventRepository(t)
	ctx := context.Background()

	exact := integrationValidatedEvent("event-exact")
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{exact}), outbound.EventPersistenceInserted)
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{exact}), outbound.EventPersistenceDuplicateExact)

	logical := integrationValidatedEvent("event-logical-original")
	logical.LogicalEventID = "logical-1"
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{logical}), outbound.EventPersistenceInserted)
	logicalDuplicate := logical
	logicalDuplicate.EventID = "event-logical-retry"
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{logicalDuplicate}), outbound.EventPersistenceDuplicateLogical)

	idempotent := integrationValidatedEvent("event-idempotent-original")
	idempotent.IdempotencyKey = "operation-1"
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{idempotent}), outbound.EventPersistenceInserted)
	idempotentDuplicate := idempotent
	idempotentDuplicate.EventID = "event-idempotent-retry"
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{idempotentDuplicate}), outbound.EventPersistenceDuplicateIdempotency)

	contradictory := idempotent
	contradictory.EventID = "event-idempotent-conflict"
	contradictory.EventName = "purchase_completed"
	valid := integrationValidatedEvent("event-valid-after-conflict")
	results := repository.SaveBatch(ctx, []event.ValidatedEvent{contradictory, valid})
	if len(results) != 2 {
		t.Fatalf("resultados = %d, want 2", len(results))
	}
	if results[0].Status != outbound.EventPersistenceConflict || results[0].ConflictField != "idempotency_key" {
		t.Fatalf("conflicto = %#v", results[0])
	}
	if results[1].Status != outbound.EventPersistenceInserted {
		t.Fatalf("evento valido = %#v", results[1])
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM analytics_events").Scan(&count); err != nil {
		t.Fatalf("contar eventos: %v", err)
	}
	if count != 4 {
		t.Fatalf("filas = %d, want 4", count)
	}
}

func TestEventRepositoryIntegrationConcurrentLogicalConflict(t *testing.T) {
	repository, _ := newIntegrationEventRepository(t)
	ctx := context.Background()
	first := integrationValidatedEvent("concurrent-1")
	first.LogicalEventID = "logical-concurrent"
	second := first
	second.EventID = "concurrent-2"

	resultChannel := make(chan outbound.EventPersistenceResult, 2)
	for _, item := range []event.ValidatedEvent{first, second} {
		item := item
		go func() {
			results := repository.SaveBatch(ctx, []event.ValidatedEvent{item})
			resultChannel <- results[0]
		}()
	}
	statuses := map[outbound.EventPersistenceStatus]int{}
	for range 2 {
		result := <-resultChannel
		if result.Err != nil {
			t.Fatalf("persistencia concurrente: %v", result.Err)
		}
		statuses[result.Status]++
	}
	if statuses[outbound.EventPersistenceInserted] != 1 || statuses[outbound.EventPersistenceDuplicateLogical] != 1 {
		t.Fatalf("estados concurrentes = %#v", statuses)
	}
}

func TestEventRepositoryIntegrationPersistsEventSource(t *testing.T) {
	repository, pool := newIntegrationEventRepository(t)
	ctx := context.Background()
	serverEvent := integrationValidatedEvent("server-event")
	serverEvent.EventSource = "server"
	serverEvent.Origin = ""
	serverEvent.URL = ""
	serverEvent.Path = ""

	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{serverEvent}), outbound.EventPersistenceInserted)
	var source string
	if err := pool.QueryRow(ctx, "SELECT event_source FROM analytics_events WHERE event_id=$1", serverEvent.EventID).Scan(&source); err != nil {
		t.Fatalf("consultar event_source: %v", err)
	}
	if source != "server" {
		t.Fatalf("event_source = %q, esperado server", source)
	}
}

func TestSubjectBootstrapIntegrationConvergesWithLazyCreation(t *testing.T) {
	events, pool := newIntegrationEventRepository(t)
	bootstrap, err := NewSubjectBootstrapRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	state, err := bootstrap.BeginOrResume(ctx)
	if err != nil {
		t.Fatal(err)
	}
	web := integrationValidatedEvent("bootstrap-lazy")
	web.SubjectDetails = []subject.EventDetail{integrationSubjectDetail("customer-bootstrap", subject.LifecycleUnknown)}
	page := subject.SnapshotPage{SourceRevision: "revision-1", Rows: []subject.SnapshotRow{{TenantID: "tenant-1", ExternalSubjectID: "customer-bootstrap", LifecycleStatus: subject.LifecycleActive, SourceUpdatedAt: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC), SourceMutationID: "snapshot-1"}}}
	errors := make(chan error, 2)
	go func() { result := events.SaveBatch(ctx, []event.ValidatedEvent{web}); errors <- result[0].Err }()
	go func() { _, applyErr := bootstrap.ApplyPage(ctx, state, page); errors <- applyErr }()
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	var count int
	var status string
	if err := pool.QueryRow(ctx, `SELECT count(*),max(lifecycle_status) FROM analytics_subjects WHERE tenant_id='tenant-1' AND external_subject_id='customer-bootstrap'`).Scan(&count, &status); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != "active" {
		t.Fatalf("count=%d status=%s", count, status)
	}
}

func TestSubjectBootstrapIntegrationDoesNotDegradeNewerLifecycle(t *testing.T) {
	events, pool := newIntegrationEventRepository(t)
	bootstrap, _ := NewSubjectBootstrapRepository(pool)
	ctx := context.Background()
	newerAt := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lifecycle := integrationValidatedEvent("lifecycle-newer-than-snapshot")
	lifecycle.EventName = "subject_deactivated"
	lifecycle.EventSource = "server"
	detail := integrationLifecycleDetail("customer-order", newerAt, "mutation-20")
	lifecycle.SubjectDetails = []subject.EventDetail{detail}
	assertPersistenceStatus(t, events.SaveBatch(ctx, []event.ValidatedEvent{lifecycle}), outbound.EventPersistenceInserted)
	state, err := bootstrap.BeginOrResume(ctx)
	if err != nil {
		t.Fatal(err)
	}
	older := subject.SnapshotPage{SourceRevision: "revision-old", Rows: []subject.SnapshotRow{{TenantID: "tenant-1", ExternalSubjectID: "customer-order", LifecycleStatus: subject.LifecycleActive, SourceUpdatedAt: newerAt.Add(-time.Hour), SourceMutationID: "mutation-99"}}}
	if _, err = bootstrap.ApplyPage(ctx, state, older); err != nil {
		t.Fatal(err)
	}
	var status, mutation string
	if err = pool.QueryRow(ctx, `SELECT lifecycle_status,lifecycle_source_mutation_id FROM analytics_subjects WHERE tenant_id='tenant-1' AND external_subject_id='customer-order'`).Scan(&status, &mutation); err != nil {
		t.Fatal(err)
	}
	if status != "inactive" || mutation != "mutation-20" {
		t.Fatalf("status=%s mutation=%s", status, mutation)
	}
}

func TestEventRepositoryIntegrationNormalizesSubjectsTenantFirst(t *testing.T) {
	repository, pool := newIntegrationEventRepository(t)
	ctx := context.Background()

	web := integrationValidatedEvent("subject-web")
	web.EventName = "subject_selected"
	web.Timezone = "America/Argentina/Buenos_Aires"
	web.SubjectDetails = []subject.EventDetail{
		integrationSubjectDetail("customer-1", subject.LifecycleUnknown),
		{SubjectOrdinal: 1, SubjectType: "account", ExternalSubjectID: "account-1", ContextResolutionStatus: subject.ContextRegistryDisabled, NormalizationStatus: subject.NormalizationUnsupportedType, NormalizationReason: "unsupported_subject_type", InitialLifecycleStatus: subject.LifecycleUnknown},
	}
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{web}), outbound.EventPersistenceInserted)

	associated := integrationValidatedEvent("subject-associated")
	associated.SiteID = "site-id-2"
	associated.SiteCode = "site-2"
	associated.EventSource = "server"
	associated.EventName = "subject_associated"
	associated.IdempotencyKey = "operation-1"
	associated.SubjectDetails = []subject.EventDetail{integrationSubjectDetail("customer-1", subject.LifecycleActive)}
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{associated}), outbound.EventPersistenceInserted)

	duplicate := associated
	duplicate.EventID = "subject-associated-retry"
	duplicate.SiteID = "site-id-3"
	duplicate.SiteCode = "site-3"
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{duplicate}), outbound.EventPersistenceInserted)

	var subjectsCount, rawCount, normalizedAssociations, conflicts, unsupported int
	var lifecycleStatus string
	if err := pool.QueryRow(ctx, "SELECT count(*), max(lifecycle_status) FROM analytics_subjects WHERE tenant_id='tenant-1' AND external_subject_id='customer-1'").Scan(&subjectsCount, &lifecycleStatus); err != nil {
		t.Fatalf("consultar identidad: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_events").Scan(&rawCount); err != nil {
		t.Fatalf("consultar eventos crudos: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_event_subjects WHERE event_name='subject_associated' AND normalization_status='normalized'").Scan(&normalizedAssociations); err != nil {
		t.Fatalf("consultar asociaciones: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_event_subjects WHERE normalization_status='idempotency_conflict'").Scan(&conflicts); err != nil {
		t.Fatalf("consultar conflictos: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_event_subjects WHERE normalization_status='unsupported_subject_type'").Scan(&unsupported); err != nil {
		t.Fatalf("consultar fallos semánticos: %v", err)
	}
	if subjectsCount != 1 || lifecycleStatus != "active" || rawCount != 3 || normalizedAssociations != 1 || conflicts != 1 || unsupported != 1 {
		t.Fatalf("subjects=%d status=%s raw=%d normalized=%d conflicts=%d unsupported=%d", subjectsCount, lifecycleStatus, rawCount, normalizedAssociations, conflicts, unsupported)
	}
}

func TestEventRepositoryIntegrationConvergesConcurrentSubjectCreation(t *testing.T) {
	repository, pool := newIntegrationEventRepository(t)
	ctx := context.Background()
	first := integrationValidatedEvent("subject-concurrent-1")
	first.SubjectDetails = []subject.EventDetail{integrationSubjectDetail("customer-concurrent", subject.LifecycleUnknown)}
	second := integrationValidatedEvent("subject-concurrent-2")
	second.SiteID = "site-id-2"
	second.SiteCode = "site-2"
	second.SubjectDetails = []subject.EventDetail{integrationSubjectDetail("customer-concurrent", subject.LifecycleUnknown)}

	resultsChannel := make(chan outbound.EventPersistenceResult, 2)
	for _, item := range []event.ValidatedEvent{first, second} {
		item := item
		go func() { resultsChannel <- repository.SaveBatch(ctx, []event.ValidatedEvent{item})[0] }()
	}
	for range 2 {
		result := <-resultsChannel
		if result.Err != nil || result.Status != outbound.EventPersistenceInserted {
			t.Fatalf("resultado concurrente = %#v", result)
		}
	}
	var subjectsCount, detailsCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_subjects WHERE external_subject_id='customer-concurrent'").Scan(&subjectsCount); err != nil {
		t.Fatalf("consultar sujetos: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_event_subjects WHERE external_subject_id='customer-concurrent'").Scan(&detailsCount); err != nil {
		t.Fatalf("consultar detalles: %v", err)
	}
	if subjectsCount != 1 || detailsCount != 2 {
		t.Fatalf("sujetos=%d detalles=%d", subjectsCount, detailsCount)
	}
}

func TestEventRepositoryIntegrationAppliesMonotonicLifecycleAndMerge(t *testing.T) {
	repository, pool := newIntegrationEventRepository(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	deactivated := integrationValidatedEvent("lifecycle-deactivated")
	deactivated.EventSource = "server"
	deactivated.EventName = "subject_deactivated"
	deactivated.IdempotencyKey = "lifecycle-20"
	deactivated.SubjectDetails = []subject.EventDetail{integrationLifecycleDetail("customer-lifecycle", base, "mutation-20")}
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{deactivated}), outbound.EventPersistenceInserted)

	older := integrationValidatedEvent("lifecycle-older")
	older.EventSource = "server"
	older.EventName = "subject_reactivated"
	older.IdempotencyKey = "lifecycle-old"
	older.SubjectDetails = []subject.EventDetail{integrationLifecycleDetail("customer-lifecycle", base.Add(-time.Second), "mutation-99")}
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{older}), outbound.EventPersistenceInserted)

	reactivated := integrationValidatedEvent("lifecycle-reactivated")
	reactivated.EventSource = "server"
	reactivated.EventName = "subject_reactivated"
	reactivated.IdempotencyKey = "lifecycle-21"
	reactivated.SubjectDetails = []subject.EventDetail{integrationLifecycleDetail("customer-lifecycle", base, "mutation-21")}
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{reactivated}), outbound.EventPersistenceInserted)

	merged := integrationValidatedEvent("lifecycle-merged")
	merged.EventSource = "server"
	merged.EventName = "subject_merged"
	merged.IdempotencyKey = "lifecycle-merge"
	mergeDetail := integrationLifecycleDetail("customer-lifecycle", base.Add(time.Second), "mutation-01")
	mergeDetail.MergedIntoExternalSubjectID = "customer-target"
	merged.SubjectDetails = []subject.EventDetail{mergeDetail}
	assertPersistenceStatus(t, repository.SaveBatch(ctx, []event.ValidatedEvent{merged}), outbound.EventPersistenceInserted)

	var status, mutation, targetExternalID string
	if err := pool.QueryRow(ctx, `
SELECT source.lifecycle_status, source.lifecycle_source_mutation_id, target.external_subject_id
FROM analytics_subjects source
JOIN analytics_subjects target ON target.id=source.merged_into_subject_id
WHERE source.external_subject_id='customer-lifecycle'`).Scan(&status, &mutation, &targetExternalID); err != nil {
		t.Fatalf("consultar merge: %v", err)
	}
	var ignored int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics_event_subjects WHERE normalization_status='lifecycle_ignored'").Scan(&ignored); err != nil {
		t.Fatalf("consultar lifecycle ignorado: %v", err)
	}
	if status != "merged" || mutation != "mutation-01" || targetExternalID != "customer-target" || ignored != 1 {
		t.Fatalf("status=%s mutation=%s target=%s ignored=%d", status, mutation, targetExternalID, ignored)
	}
}

func newIntegrationEventRepository(t *testing.T) (*EventRepository, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_INTEGRATION_DSN no configurada")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("conectar postgres: %v", err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("task_ca_0002_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("crear schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`) })

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsear dsn: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("crear pool aislado: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, integrationEventSchemaSQL); err != nil {
		t.Fatalf("crear tablas: %v", err)
	}
	repository, err := NewEventRepository(pool)
	if err != nil {
		t.Fatalf("NewEventRepository() error = %v", err)
	}
	return repository, pool
}

func integrationValidatedEvent(eventID string) event.ValidatedEvent {
	return event.ValidatedEvent{
		EventID: eventID, TenantID: "tenant-1", SiteID: "site-id-1", SiteCode: "site-1",
		Environment: "production", EventName: "page_viewed", EventVersion: 1,
		EventTime:   time.Date(2026, 8, 7, 12, 0, 0, 123456000, time.UTC),
		ReceivedAt:  time.Date(2026, 8, 7, 12, 0, 1, 0, time.UTC),
		AnonymousID: "anonymous-1", SessionID: "session-1", Origin: "https://example.com",
		URL: "https://example.com/products", Path: "/products",
		Properties: map[string]any{"surface": "catalog"}, Context: map[string]any{"locale": "es-AR"},
	}
}

func integrationSubjectDetail(externalID string, status subject.LifecycleStatus) subject.EventDetail {
	localDate := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	return subject.EventDetail{
		SubjectOrdinal:          0,
		SubjectType:             "customer",
		ExternalSubjectID:       externalID,
		Relation:                "selected",
		ContextResolutionStatus: subject.ContextRegistryDisabled,
		TimezoneName:            "America/Argentina/Buenos_Aires",
		LocalDate:               &localDate,
		NormalizationStatus:     subject.NormalizationNormalized,
		Metadata:                map[string]any{},
		InitialLifecycleStatus:  status,
	}
}

func integrationLifecycleDetail(externalID string, updatedAt time.Time, mutationID string) subject.EventDetail {
	detail := integrationSubjectDetail(externalID, subject.LifecycleUnknown)
	detail.Relation = "lifecycle"
	detail.LifecycleSourceUpdatedAt = &updatedAt
	detail.LifecycleSourceMutationID = mutationID
	return detail
}

func assertPersistenceStatus(t *testing.T, results []outbound.EventPersistenceResult, want outbound.EventPersistenceStatus) {
	t.Helper()
	if len(results) != 1 || results[0].Status != want || results[0].Err != nil {
		t.Fatalf("resultados = %#v, want %s", results, want)
	}
}

const integrationEventSchemaSQL = `
CREATE TABLE analytics_events (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL,
    logical_event_id TEXT,
    idempotency_key TEXT,
    tab_id TEXT,
    sequence BIGINT,
    previous_logical_event_id TEXT,
    dedup_strategy TEXT,
    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    site_code TEXT NOT NULL,
	    env TEXT NOT NULL,
	    event_source TEXT NOT NULL DEFAULT 'web',
    event_name TEXT NOT NULL,
    event_version INTEGER NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    anonymous_id TEXT,
    user_id TEXT,
    session_id TEXT,
    origin TEXT,
    url TEXT,
    path TEXT,
    referrer TEXT,
    user_agent TEXT,
    ip_hash TEXT,
    sdk_name TEXT,
    sdk_version TEXT,
    jwt_id TEXT,
    token_version INTEGER,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    context JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE UNIQUE INDEX ux_events_event_id ON analytics_events(event_id);
CREATE UNIQUE INDEX ux_events_logical ON analytics_events(tenant_id, site_id, logical_event_id)
    WHERE logical_event_id IS NOT NULL AND logical_event_id <> '';
CREATE UNIQUE INDEX ux_events_idempotency ON analytics_events(tenant_id, site_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
CREATE TABLE analytics_event_items (analytics_event_id BIGINT NOT NULL);
CREATE TABLE analytics_event_orders (analytics_event_id BIGINT NOT NULL);
CREATE TABLE analytics_subjects (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    subject_type TEXT NOT NULL CHECK (subject_type = 'customer'),
    external_subject_id TEXT NOT NULL,
    lifecycle_status TEXT NOT NULL CHECK (lifecycle_status IN ('unknown','active','inactive','merged')),
    lifecycle_source_updated_at TIMESTAMPTZ,
    lifecycle_source_mutation_id TEXT,
    lifecycle_effective_at TIMESTAMPTZ,
    merged_into_subject_id BIGINT REFERENCES analytics_subjects(id),
    status_updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, subject_type, external_subject_id)
);
CREATE TABLE analytics_event_subjects (
    id BIGSERIAL PRIMARY KEY,
    analytics_event_id BIGINT NOT NULL REFERENCES analytics_events(id) ON DELETE CASCADE,
    subject_ordinal INTEGER NOT NULL,
    client_event_id TEXT NOT NULL,
    logical_event_id TEXT,
    idempotency_key TEXT,
    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    event_source TEXT NOT NULL,
    event_name TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    analytics_subject_id BIGINT REFERENCES analytics_subjects(id),
    subject_type TEXT NOT NULL,
    external_subject_id TEXT NOT NULL,
    relation TEXT NOT NULL DEFAULT '',
    operation_type TEXT,
    operation_id TEXT,
    context_definition_id BIGINT,
    context_key TEXT,
    context_version INTEGER,
    claimed_context_family TEXT,
    resolved_context_family TEXT,
    context_resolution_status TEXT NOT NULL,
    context_registry_revision_id BIGINT,
    timezone_name TEXT,
    local_date DATE,
    normalization_status TEXT NOT NULL,
    normalization_reason TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (analytics_event_id, subject_ordinal)
);
CREATE UNIQUE INDEX ux_event_subjects_tenant_idempotency
ON analytics_event_subjects(tenant_id,event_name,idempotency_key,subject_type,external_subject_id,relation)
WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
CREATE TABLE analytics_subject_bootstrap (
    id BIGSERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    tenant_id TEXT,
    status TEXT NOT NULL,
    cursor TEXT,
    source_revision TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    processed_count BIGINT NOT NULL DEFAULT 0,
    failed_count BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_subject_bootstrap_running ON analytics_subject_bootstrap(subject_type) WHERE status='running';
`
