package redis

import (
	"context"
	goredis "github.com/redis/go-redis/v9"
	"goanalytics/services/worker/internal/domain/subject"
	"os"
	"testing"
	"time"
)

func TestContextRegistryCacheIntegrationTokenizedLock(t *testing.T) {
	address := os.Getenv("REDIS_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("REDIS_INTEGRATION_ADDR no configurada")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: address})
	defer client.Close()
	defer client.Del(ctx, registryCacheKey, registryLockKey)
	cache := NewContextRegistryCache(client, time.Minute, 30*time.Second)
	registry := subject.ContextRegistry{ID: 9, SchemaVersion: 1, RegistryVersion: 1, RegistryRevision: "r1", Contexts: []subject.ContextDefinition{{ID: 7, ContextKey: "search", ContextVersion: 1, ContextFamily: "search", Status: "active"}}}
	if err := cache.Save(ctx, registry); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := cache.Load(ctx)
	if err != nil || !ok || loaded.RegistryRevision != "r1" || loaded.ID != 9 || loaded.Contexts[0].ID != 7 || loaded.PayloadChecksum == "" {
		t.Fatalf("loaded=%#v ok=%v err=%v", loaded, ok, err)
	}
	token, acquired, err := cache.TryAcquire(ctx)
	if err != nil || !acquired {
		t.Fatal(err)
	}
	if err = cache.Release(ctx, "otro-token"); err != nil {
		t.Fatal(err)
	}
	if _, second, _ := cache.TryAcquire(ctx); second {
		t.Fatal("token ajeno liberó lock")
	}
	if err = cache.Release(ctx, token); err != nil {
		t.Fatal(err)
	}
	_, third, err := cache.TryAcquire(ctx)
	if err != nil || !third {
		t.Fatalf("third=%v err=%v", third, err)
	}
}
