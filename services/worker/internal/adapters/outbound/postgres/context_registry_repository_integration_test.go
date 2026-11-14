package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"goanalytics/services/worker/internal/domain/subject"
)

// migratedPool sustituye solo el schema efímero del fixture por migraciones reales.
func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_, pool := newIntegrationEventRepository(t)
	ctx := context.Background()
	var schema string
	if err := pool.QueryRow(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatal(err)
	}
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE; CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "..", "..", "migrations", "postgres", "*.up.sql"))
	if err != nil || len(paths) != 8 {
		t.Fatalf("migraciones=%v err=%v", paths, err)
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(content)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	return pool
}

func TestRegistryIntegrationPublishesImmutableIncreasingRevisions(t *testing.T) {
	pool := migratedPool(t)
	repository, _ := NewContextRegistryRepository(pool)
	ctx := context.Background()
	registry := subject.ContextRegistry{SchemaVersion: 1, RegistryVersion: 1, RegistryRevision: "r1", Contexts: []subject.ContextDefinition{{ContextKey: "root", ContextVersion: 1, ContextFamily: "search", Status: "active"}, {ContextKey: "child", ContextVersion: 1, ContextFamily: "selection", Status: "active"}}}
	first, err := repository.PersistValid(ctx, registry)
	if err != nil || first.ID == 0 || first.Contexts[0].ID == 0 {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	loaded, found, err := repository.LoadActive(ctx)
	if err != nil || !found || loaded.ID != first.ID || loaded.Contexts[0].ID != first.Contexts[0].ID {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	registry.RegistryVersion = 2
	registry.RegistryRevision = "r2"
	registry.Contexts[0].Status = "inactive"
	if _, err = repository.PersistValid(ctx, registry); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.PersistValid(ctx, first); err == nil {
		t.Fatal("se activó revisión anterior")
	}
	key, version := "root", 1
	registry.RegistryVersion = 3
	registry.RegistryRevision = "r3"
	registry.Contexts[1].ParentContextKey = &key
	registry.Contexts[1].ParentContextVersion = &version
	if _, err = repository.PersistValid(ctx, registry); err == nil {
		t.Fatal("se alteró padre publicado")
	}
	loaded, found, err = repository.LoadActive(ctx)
	if err != nil || !found || loaded.RegistryRevision != "r2" {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	queries, _ := NewSuggestionRepository(pool)
	status, statusErr := queries.ContextResolutionStatus(ctx, "root", 1)
	if statusErr != nil || status != "inactive_context" {
		t.Fatalf("status=%s err=%v", status, statusErr)
	}
	results, registered, err := queries.ContextSuggestions(ctx, "tenant", "root", 1, 1, 30, 10)
	if err != nil || registered || len(results) > 0 {
		t.Fatalf("inactive=%v registered=%v err=%v", results, registered, err)
	}
}
