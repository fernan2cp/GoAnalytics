package usecases

import (
	"context"
	"errors"
	"goanalytics/services/worker/internal/domain/subject"
	"testing"
)

type registryCacheFake struct {
	value subject.ContextRegistry
	ok    bool
	saved int
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
