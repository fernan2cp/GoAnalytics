package usecases

import (
	"context"
	"sync"
	"time"

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
	expiresAt  time.Time
	ttl        time.Duration
	metrics    outbound.Metrics
}

// WithMetrics conecta observabilidad opcional antes de compartir el cargador.
// Recibe el puerto numérico y devuelve la misma instancia; no realiza I/O.
func (loader *ContextRegistryLoader) WithMetrics(metrics outbound.Metrics) *ContextRegistryLoader {
	loader.metrics = metrics
	return loader
}

// NewContextRegistryLoader crea el cargador revisionado.
// Recibe caché, lock, repositorio, fuente y TTL opcional de memoria (un minuto
// por defecto). Devuelve una instancia sin I/O; las dependencias son obligatorias
// y sus errores se devuelven al cargar o refrescar el registro.
func NewContextRegistryLoader(cache outbound.ContextRegistryCache, lock outbound.ContextRegistryLock, repository outbound.ContextRegistryRepository, source outbound.ContextRegistrySource, ttl ...time.Duration) *ContextRegistryLoader {
	lifetime := time.Minute
	if len(ttl) > 0 && ttl[0] > 0 {
		lifetime = ttl[0]
	}
	return &ContextRegistryLoader{cache: cache, lock: lock, repository: repository, source: source, ttl: lifetime}
}

// Active devuelve memoria o rehidrata desde las capas acordadas.
// Recibe contexto y devuelve una revisión usable o error si ninguna fuente está
// disponible. La memoria vencida consulta la fuente bajo lock distribuido.
func (loader *ContextRegistryLoader) Active(ctx context.Context) (subject.ContextRegistry, error) {
	loader.mu.RLock()
	if loader.active != nil && time.Now().Before(loader.expiresAt) {
		value := *loader.active
		loader.mu.RUnlock()
		return value, nil
	}
	expired := loader.active != nil
	loader.mu.RUnlock()
	if expired {
		return loader.Refresh(ctx)
	}
	if cached, ok, err := loader.cache.Load(ctx); err == nil && ok {
		loader.recordLoad("cache")
		return loader.publish(cached), nil
	}
	if durable, ok, err := loader.repository.LoadActive(ctx); err == nil && ok {
		loader.recordLoad("durable")
		_ = loader.cache.Save(ctx, durable)
		return loader.publish(durable), nil
	}
	return loader.Refresh(ctx)
}

// Refresh consulta la fuente aunque exista caché y conserva la última revisión
// válida ante fallos o contención. Recibe contexto y devuelve la revisión usable
// o un error si ninguna capa permite continuar; usa exclusión distribuida.
func (loader *ContextRegistryLoader) Refresh(ctx context.Context) (subject.ContextRegistry, error) {
	token, acquired, err := loader.lock.TryAcquire(ctx)
	if err != nil {
		loader.recordLoad("lock_error")
		return loader.fallback(ctx, err)
	}
	if !acquired {
		loader.recordLoad("locked")
		return loader.fallback(ctx, ErrRegistryUnavailable)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = loader.lock.Release(releaseCtx, token)
	}()
	registry, err := loader.source.Fetch(ctx)
	if err != nil {
		loader.recordLoad("source_error")
		return loader.fallback(ctx, err)
	}
	if err = registry.Validate(); err != nil {
		loader.recordLoad("invalid")
		_ = loader.repository.RecordInvalid(ctx, registry, err.Error())
		return loader.fallback(ctx, err)
	}
	registry, err = loader.repository.PersistValid(ctx, registry)
	if err != nil {
		loader.recordLoad("rejected")
		return loader.fallback(ctx, err)
	}
	_ = loader.cache.Save(ctx, registry)
	loader.recordLoad("loaded")
	return loader.publish(registry), nil
}

// recordLoad clasifica resultados sin etiquetas de tenant o sujeto.
func (loader *ContextRegistryLoader) recordLoad(status string) {
	if loader.metrics != nil {
		loader.metrics.IncrementCounter("subject_context_registry_loads_total", map[string]string{"status": status})
	}
}

func (loader *ContextRegistryLoader) publish(registry subject.ContextRegistry) subject.ContextRegistry {
	loader.mu.Lock()
	loader.active = &registry
	loader.expiresAt = time.Now().Add(loader.ttl)
	loader.mu.Unlock()
	return registry
}

// fallback prefiere la revisión durable activa a una copia local anterior.
func (loader *ContextRegistryLoader) fallback(ctx context.Context, cause error) (subject.ContextRegistry, error) {
	if value, ok, _ := loader.repository.LoadActive(ctx); ok {
		return loader.publish(value), nil
	}
	loader.mu.RLock()
	if loader.active != nil {
		value := *loader.active
		loader.mu.RUnlock()
		return loader.publish(value), nil
	}
	loader.mu.RUnlock()
	return subject.ContextRegistry{}, cause
}

// ErrRegistryUnavailable indica que otro proceso refresca y no existe fallback.
var ErrRegistryUnavailable = registryUnavailableError{}

type registryUnavailableError struct{}

func (registryUnavailableError) Error() string { return "registro de contextos no disponible" }
