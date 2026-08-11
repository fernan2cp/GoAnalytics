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
`
