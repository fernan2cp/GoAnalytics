package usecases

import (
	"context"
	"sync"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/subject"
)

// ContextRegistryLoader implementa memoria → Redis → PostgreSQL → fuente.
type ContextRegistryLoader struct {
	cache      outbound.ContextRegistryCache
	lock       outbound.ContextRegistryLock
	repository outbound.ContextRegistryRepository
	source     outbound.ContextRegistrySource
	mu         sync.RWMutex
	active     *subject.ContextRegistry
}

// NewContextRegistryLoader crea el cargador revisionado.
func NewContextRegistryLoader(cache outbound.ContextRegistryCache, lock outbound.ContextRegistryLock, repository outbound.ContextRegistryRepository, source outbound.ContextRegistrySource) *ContextRegistryLoader {
	return &ContextRegistryLoader{cache: cache, lock: lock, repository: repository, source: source}
}

// Active devuelve memoria o rehidrata desde las capas acordadas.
func (loader *ContextRegistryLoader) Active(ctx context.Context) (subject.ContextRegistry, error) {
	loader.mu.RLock()
	if loader.active != nil {
		value := *loader.active
		loader.mu.RUnlock()
		return value, nil
	}
	loader.mu.RUnlock()
	if cached, ok, err := loader.cache.Load(ctx); err == nil && ok {
		return loader.publish(cached), nil
	}
	if durable, ok, err := loader.repository.LoadActive(ctx); err == nil && ok {
		_ = loader.cache.Save(ctx, durable)
		return loader.publish(durable), nil
	}
	token, acquired, err := loader.lock.TryAcquire(ctx)
	if err != nil {
		return subject.ContextRegistry{}, err
	}
	if !acquired {
		return subject.ContextRegistry{}, ErrRegistryUnavailable
	}
	defer loader.lock.Release(context.Background(), token)
	registry, err := loader.source.Fetch(ctx)
	if err != nil {
		return subject.ContextRegistry{}, err
	}
	if err = registry.Validate(); err != nil {
		_ = loader.repository.RecordInvalid(ctx, registry, err.Error())
		if fallback, ok, _ := loader.repository.LoadActive(ctx); ok {
			return loader.publish(fallback), nil
		}
		return subject.ContextRegistry{}, err
	}
	registry, err = loader.repository.PersistValid(ctx, registry)
	if err != nil {
		if fallback, ok, _ := loader.repository.LoadActive(ctx); ok {
			return loader.publish(fallback), nil
		}
		return subject.ContextRegistry{}, err
	}
	_ = loader.cache.Save(ctx, registry)
	return loader.publish(registry), nil
}

func (loader *ContextRegistryLoader) publish(registry subject.ContextRegistry) subject.ContextRegistry {
	loader.mu.Lock()
	loader.active = &registry
	loader.mu.Unlock()
	return registry
}

// ErrRegistryUnavailable indica que otro proceso refresca y no existe fallback.
var ErrRegistryUnavailable = registryUnavailableError{}

type registryUnavailableError struct{}

func (registryUnavailableError) Error() string { return "registro de contextos no disponible" }
