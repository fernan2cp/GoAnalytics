package outbound

import (
	"context"

	"goanalytics/services/ingest/internal/domain/event"
)

// EventPublisher define el puerto para publicar eventos crudos.
//
// Recibe un contexto y una lista de event.RawEvent ya aceptados por el caso de
// uso. No devuelve datos; devuelve error cuando la cola, stream o transporte
// de publicacion falla.
//
// Debe implementarse en adaptadores outbound, por ejemplo Redis Stream, Kafka
// o Redpanda. La aplicacion no debe depender de esas tecnologias concretas.
type EventPublisher interface {
	Publish(ctx context.Context, events []event.RawEvent) error
}
