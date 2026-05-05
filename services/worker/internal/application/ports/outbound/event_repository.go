package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/event"
)

// EventRepository define el puerto para persistir eventos validos.
//
// Recibe un contexto y una lista de event.ValidatedEvent. No devuelve datos;
// devuelve error cuando la base de datos rechaza la operacion o no esta
// disponible.
//
// Debe implementarse en adaptadores outbound, inicialmente PostgreSQL y en el
// futuro ClickHouse sin modificar los casos de uso.
type EventRepository interface {
	SaveBatch(ctx context.Context, events []event.ValidatedEvent) error
}
