package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/subject"
)

// ContextRegistrySource obtiene el registro desde el proyecto principal.
type ContextRegistrySource interface {
	Fetch(ctx context.Context) (subject.ContextRegistry, error)
}

// ContextRegistryCache almacena la revisión activa en Redis.
type ContextRegistryCache interface {
	Load(ctx context.Context) (subject.ContextRegistry, bool, error)
	Save(ctx context.Context, registry subject.ContextRegistry) error
}

// ContextRegistryLock coordina refresh distribuido con token propietario.
type ContextRegistryLock interface {
	TryAcquire(ctx context.Context) (string, bool, error)
	Release(ctx context.Context, token string) error
}

// ContextRegistryRepository conserva revisiones y definiciones inmutables.
type ContextRegistryRepository interface {
	LoadActive(ctx context.Context) (subject.ContextRegistry, bool, error)
	PersistValid(ctx context.Context, registry subject.ContextRegistry) (subject.ContextRegistry, error)
	RecordInvalid(ctx context.Context, registry subject.ContextRegistry, cause string) error
}
