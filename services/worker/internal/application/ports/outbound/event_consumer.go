package outbound

import (
	"context"

	"goanalytics/services/worker/internal/application/dto"
)

// EventConsumer define el puerto outbound para leer y confirmar eventos.
//
// ConsumeBatch recibe un contexto y un limite de lectura; devuelve eventos
// crudos o error si la cola falla. Ack recibe eventos procesados y confirma su
// procesamiento; devuelve error si la confirmacion falla.
//
// Debe implementarse en adaptadores outbound de cola o stream. La aplicacion
// no debe conocer Redis Streams, Kafka ni otros transportes concretos.
type EventConsumer interface {
	ConsumeBatch(ctx context.Context, limit int) ([]dto.RawEvent, error)
	Ack(ctx context.Context, events []dto.RawEvent) error
}
