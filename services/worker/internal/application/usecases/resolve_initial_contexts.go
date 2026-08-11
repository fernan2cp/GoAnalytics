package usecases

import (
	"context"
	"fmt"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

// ResolveInitialContexts resuelve una sola vez filas anteriores al registro.
type ResolveInitialContexts struct {
	repository outbound.ContextResolutionRepository
	registry   *ContextRegistryLoader
	batchSize  int
}

// NewResolveInitialContexts construye el job paginado e idempotente.
func NewResolveInitialContexts(repository outbound.ContextResolutionRepository, registry *ContextRegistryLoader, batchSize int) (*ResolveInitialContexts, error) {
	if repository == nil || registry == nil || batchSize < 1 {
		return nil, fmt.Errorf("configuración de resolución inicial inválida")
	}
	return &ResolveInitialContexts{repository: repository, registry: registry, batchSize: batchSize}, nil
}

// Run resuelve páginas hasta agotar filas sin revisión.
func (useCase *ResolveInitialContexts) Run(ctx context.Context) error {
	registry, err := useCase.registry.Active(ctx)
	if err != nil {
		return err
	}
	afterID, err := useCase.repository.LoadContextResolutionCheckpoint(ctx, registry.ID)
	if err != nil {
		return err
	}
	for {
		processed, lastID, resolveErr := useCase.repository.ResolveContextBatch(ctx, registry.ID, afterID, useCase.batchSize)
		if resolveErr != nil {
			return resolveErr
		}
		if processed == 0 {
			return nil
		}
		afterID = lastID
		if processed < int64(useCase.batchSize) {
			return nil
		}
	}
}
