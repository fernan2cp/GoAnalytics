package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/rejection"
)

// RejectedEventRepository define el puerto para persistir rechazos.
//
// Recibe un contexto y una lista de rejection.RejectedEvent. No devuelve datos;
// devuelve error cuando el almacenamiento de auditoria falla.
//
// Debe usarse para eventos invalidos, sospechosos o no resolubles sin detener
// el procesamiento de otros eventos del batch.
type RejectedEventRepository interface {
	SaveBatch(ctx context.Context, events []rejection.RejectedEvent) error
}
