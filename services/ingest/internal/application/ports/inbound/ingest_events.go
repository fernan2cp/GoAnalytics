package inbound

import (
	"context"

	"goanalytics/services/ingest/internal/application/dto"
)

// IngestEvents define el puerto inbound del caso de uso de ingesta.
//
// Recibe un contexto y un dto.IngestRequest construido por un adaptador
// externo. Devuelve dto.IngestResponse con el resultado de aceptacion y error
// cuando la solicitud no puede procesarse.
//
// Debe implementarse en la capa application. Los adaptadores inbound, como
// HTTP, dependen de esta interfaz para no conocer la implementacion concreta.
type IngestEvents interface {
	Ingest(ctx context.Context, request dto.IngestRequest) (dto.IngestResponse, error)
}
