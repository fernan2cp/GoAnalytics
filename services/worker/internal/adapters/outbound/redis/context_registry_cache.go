package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"goanalytics/services/worker/internal/domain/subject"
)

const registryCacheKey = "goanalytics:subject-context-registry:active:v2"
const registryLockKey = "goanalytics:subject-context-registry:rehydrate-lock"

// ContextRegistryCache implementa caché y lock tokenizado en Redis.
type ContextRegistryCache struct {
	client       *goredis.Client
	ttl, lockTTL time.Duration
}

// registryCachePayload conserva IDs técnicos exclusivamente en la caché interna.
type registryCachePayload struct {
	Registry      subject.ContextRegistry `json:"registry"`
	RevisionID    int64                   `json:"revision_id"`
	DefinitionIDs []int64                 `json:"definition_ids"`
}

// NewContextRegistryCache crea el adaptador Redis.
func NewContextRegistryCache(client *goredis.Client, ttl, lockTTL time.Duration) *ContextRegistryCache {
	return &ContextRegistryCache{client: client, ttl: ttl, lockTTL: lockTTL}
}
func (cache *ContextRegistryCache) Load(ctx context.Context) (subject.ContextRegistry, bool, error) {
	value, err := cache.client.Get(ctx, registryCacheKey).Bytes()
	if err == goredis.Nil {
		return subject.ContextRegistry{}, false, nil
	}
	if err != nil {
		return subject.ContextRegistry{}, false, err
	}
	var payload registryCachePayload
	if err = json.Unmarshal(value, &payload); err != nil {
		return subject.ContextRegistry{}, false, err
	}
	registry := payload.Registry
	if payload.RevisionID <= 0 || len(payload.DefinitionIDs) != len(registry.Contexts) {
		return registry, false, nil
	}
	registry.ID = payload.RevisionID
	for index := range registry.Contexts {
		if payload.DefinitionIDs[index] <= 0 {
			return registry, false, nil
		}
		registry.Contexts[index].ID = payload.DefinitionIDs[index]
	}
	if err = registry.Validate(); err != nil {
		return registry, false, err
	}
	return registry, true, nil
}
func (cache *ContextRegistryCache) Save(ctx context.Context, registry subject.ContextRegistry) error {
	ids := make([]int64, len(registry.Contexts))
	for index := range registry.Contexts {
		ids[index] = registry.Contexts[index].ID
	}
	payload, err := json.Marshal(registryCachePayload{Registry: registry, RevisionID: registry.ID, DefinitionIDs: ids})
	if err != nil {
		return err
	}
	return cache.client.Set(ctx, registryCacheKey, payload, cache.ttl).Err()
}
func (cache *ContextRegistryCache) TryAcquire(ctx context.Context) (string, bool, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", false, err
	}
	token := hex.EncodeToString(bytes)
	ok, err := cache.client.SetNX(ctx, registryLockKey, token, cache.lockTTL).Result()
	return token, ok, err
}
func (cache *ContextRegistryCache) Release(ctx context.Context, token string) error {
	return cache.client.Eval(ctx, `if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end`, []string{registryLockKey}, token).Err()
}
