package usecases

import (
	"context"
	"errors"
	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/subject"
	"testing"
	"time"
)

type registryCacheFake struct {
	value subject.ContextRegistry
	ok    bool
	saved int
}

func TestResolveSubjectContextsClassifiesExactInactiveAndUnknown(t *testing.T) {
	version := 1
	registry := validRegistry()
	registry.ID = 9
	registry.Contexts[0].ID = 7
	registry.Contexts = append(registry.Contexts, subject.ContextDefinition{ID: 8, ContextKey: "support", ContextVersion: 1, ContextFamily: "support", Status: "inactive"})
	loader := NewContextRegistryLoader(&registryCacheFake{value: registry, ok: true}, &registryLockFake{}, &registryRepoFake{}, &registrySourceFake{})
	useCase := &ProcessEventsUseCase{contextRegistry: loader}
	value := event.ValidatedEvent{Context: map[string]any{"_goanalytics": map[string]any{}}, SubjectDetails: []subject.EventDetail{
		{ContextKey: "search", ContextVersion: &version, NormalizationStatus: subject.NormalizationNormalized},
		{ContextKey: "support", ContextVersion: &version, NormalizationStatus: subject.NormalizationNormalized},
		{ContextKey: "missing", ContextVersion: &version, NormalizationStatus: subject.NormalizationNormalized},
	}}
	useCase.resolveSubjectContexts(context.Background(), &value)
	if value.SubjectDetails[0].ContextResolutionStatus != subject.ContextRegistered || *value.SubjectDetails[0].ContextDefinitionID != 7 {
		t.Fatalf("contexto activo = %#v", value.SubjectDetails[0])
	}
	if value.SubjectDetails[1].ContextResolutionStatus != subject.ContextInactive || value.SubjectDetails[2].ContextResolutionStatus != subject.ContextUnregistered {
		t.Fatalf("clasificaciones = %#v", value.SubjectDetails)
	}
	if value.Context["_goanalytics"].(map[string]any)["context_registry_revision"] != "r1" {
		t.Fatalf("snapshot = %#v", value.Context)
	}
}

func (fake *registryCacheFake) Load(context.Context) (subject.ContextRegistry, bool, error) {
	return fake.value, fake.ok, nil
}
func (fake *registryCacheFake) Save(context.Context, subject.ContextRegistry) error {
	fake.saved++
	return nil
}

type registryLockFake struct {
	acquired bool
	released bool
}

func (fake *registryLockFake) TryAcquire(context.Context) (string, bool, error) {
	return "token", fake.acquired, nil
}
func (fake *registryLockFake) Release(context.Context, string) error {
	fake.released = true
	return nil
}

type registryRepoFake struct {
	active    subject.ContextRegistry
	ok        bool
	firstMiss bool
	loads     int
	persisted int
	invalid   int
}

func (fake *registryRepoFake) LoadActive(context.Context) (subject.ContextRegistry, bool, error) {
	fake.loads++
	if fake.firstMiss && fake.loads == 1 {
		return subject.ContextRegistry{}, false, nil
	}
	return fake.active, fake.ok, nil
}
func (fake *registryRepoFake) PersistValid(_ context.Context, value subject.ContextRegistry) (subject.ContextRegistry, error) {
	fake.persisted++
	return value, nil
}
func (fake *registryRepoFake) RecordInvalid(context.Context, subject.ContextRegistry, string) error {
	fake.invalid++
	return nil
}

type registrySourceFake struct {
	value subject.ContextRegistry
	err   error
}

func (fake *registrySourceFake) Fetch(context.Context) (subject.ContextRegistry, error) {
	return fake.value, fake.err
}
func validRegistry() subject.ContextRegistry {
	return subject.ContextRegistry{SchemaVersion: 1, RegistryVersion: 1, RegistryRevision: "r1", Contexts: []subject.ContextDefinition{{ContextKey: "search", ContextVersion: 1, ContextFamily: "search", Status: "active"}}}
}
func TestContextRegistryLoaderUsesDurableBeforeSource(t *testing.T) {
	cache := &registryCacheFake{}
	repo := &registryRepoFake{active: validRegistry(), ok: true}
	source := &registrySourceFake{err: errors.New("no debería llamarse")}
	loader := NewContextRegistryLoader(cache, &registryLockFake{}, repo, source)
	value, err := loader.Active(context.Background())
	if err != nil || value.RegistryRevision != "r1" || cache.saved != 1 {
		t.Fatalf("value=%#v err=%v saved=%d", value, err, cache.saved)
	}
}
func TestContextRegistryLoaderKeepsLastValidOnInvalidSource(t *testing.T) {
	fallback := validRegistry()
	repo := &registryRepoFake{active: fallback, ok: true, firstMiss: true}
	source := &registrySourceFake{value: subject.ContextRegistry{SchemaVersion: 1, RegistryVersion: 2, RegistryRevision: "r2"}}
	lock := &registryLockFake{acquired: true}
	loader := NewContextRegistryLoader(&registryCacheFake{}, lock, repo, source)
	value, err := loader.Active(context.Background())
	if err != nil || value.RegistryRevision != "r1" || repo.invalid != 1 || !lock.released {
		t.Fatalf("value=%#v err=%v repo=%#v lock=%#v", value, err, repo, lock)
	}
}

func TestRegistryRefreshReplacesWarmMemoryAndExpires(t *testing.T) {
	initial := validRegistry()
	newer := validRegistry()
	newer.RegistryVersion = 2
	newer.RegistryRevision = "r2"
	source := &registrySourceFake{value: newer}
	repo := &registryRepoFake{active: initial, ok: true}
	loader := NewContextRegistryLoader(&registryCacheFake{}, &registryLockFake{acquired: true}, repo, source)
	if _, err := loader.Active(context.Background()); err != nil {
		t.Fatal(err)
	}
	value, err := loader.Refresh(context.Background())
	if err != nil || value.RegistryRevision != "r2" || repo.persisted != 1 {
		t.Fatalf("value=%#v err=%v", value, err)
	}
	newer.RegistryRevision = "r3"
	newer.RegistryVersion = 3
	source.value = newer
	loader.expiresAt = time.Now().Add(-time.Second)
	value, err = loader.Active(context.Background())
	if err != nil || value.RegistryRevision != "r3" {
		t.Fatalf("value=%#v err=%v", value, err)
	}
}

func TestRegistryRefreshPreservesValidRevisionOnSourceFailure(t *testing.T) {
	initial := validRegistry()
	loader := NewContextRegistryLoader(&registryCacheFake{}, &registryLockFake{acquired: true}, &registryRepoFake{active: initial, ok: true}, &registrySourceFake{err: errors.New("fuente caída")})
	value, err := loader.Refresh(context.Background())
	if err != nil || value.RegistryRevision != "r1" {
		t.Fatalf("value=%#v err=%v", value, err)
	}
}
