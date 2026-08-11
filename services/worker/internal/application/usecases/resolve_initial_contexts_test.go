package usecases

import (
	"context"
	"testing"
)

type contextResolutionFake struct {
	checkpoint int64
	pages      []int64
	calls      int
}

func (fake *contextResolutionFake) LoadContextResolutionCheckpoint(context.Context, int64) (int64, error) {
	return fake.checkpoint, nil
}
func (fake *contextResolutionFake) ResolveContextBatch(_ context.Context, _ int64, after int64, _ int) (int64, int64, error) {
	fake.calls++
	if len(fake.pages) == 0 {
		return 0, after, nil
	}
	count := fake.pages[0]
	fake.pages = fake.pages[1:]
	return count, after + count, nil
}

func TestResolveInitialContextsResumesAndStopsOnShortPage(t *testing.T) {
	registry := validRegistry()
	registry.ID = 4
	loader := NewContextRegistryLoader(&registryCacheFake{value: registry, ok: true}, &registryLockFake{}, &registryRepoFake{}, &registrySourceFake{})
	repository := &contextResolutionFake{checkpoint: 10, pages: []int64{2}}
	job, err := NewResolveInitialContexts(repository, loader, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = job.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 {
		t.Fatalf("calls=%d", repository.calls)
	}
}

func TestResolveInitialContextsSecondRunIsNoOp(t *testing.T) {
	registry := validRegistry()
	registry.ID = 4
	loader := NewContextRegistryLoader(&registryCacheFake{value: registry, ok: true}, &registryLockFake{}, &registryRepoFake{}, &registrySourceFake{})
	repository := &contextResolutionFake{checkpoint: 20}
	job, _ := NewResolveInitialContexts(repository, loader, 100)
	if err := job.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 {
		t.Fatalf("calls=%d", repository.calls)
	}
}
