package inbound

import (
	"context"

	"goanalytics/services/ingest/internal/application/dto"
)

// IngestInternalEvents define el puerto inbound de eventos server-to-server.
type IngestInternalEvents interface {
	// IngestInternal valida y publica un batch interno en el stream compartido.
	IngestInternal(ctx context.Context, request dto.InternalIngestRequest) (dto.IngestResponse, error)
}
