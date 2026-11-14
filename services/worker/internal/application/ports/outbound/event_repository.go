package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/event"
)

// EventPersistenceStatus clasifica el resultado durable de un evento.
//
// Permite que application distinga inserciones, duplicados esperables y usos
// contradictorios de una clave sin depender de codigos PostgreSQL.
type EventPersistenceStatus string

// Estados estables devueltos por EventRepository.
const (
	EventPersistenceInserted             EventPersistenceStatus = "inserted"
	EventPersistenceDuplicateExact       EventPersistenceStatus = "duplicate_exact"
	EventPersistenceDuplicateLogical     EventPersistenceStatus = "duplicate_logical"
	EventPersistenceDuplicateIdempotency EventPersistenceStatus = "duplicate_idempotency"
	EventPersistenceConflict             EventPersistenceStatus = "conflict"
)

// EventPersistenceResult contiene el resultado individual de persistencia.
//
// Identifica el evento entrante, el estado estable, la fila preexistente y la
// clave que produjo un conflicto. Err representa una falla no clasificable que
// debe dejar el mensaje Redis pendiente.
type EventPersistenceResult struct {
	EventID         string
	Status          EventPersistenceStatus
	ExistingEventID string
	ConflictField   string
	Err             error
}

// IsIdempotentSuccess indica si el resultado permite confirmar el evento.
//
// No recibe parametros. Devuelve true para una insercion o duplicado esperado;
// devuelve false para conflictos contradictorios y errores de infraestructura.
func (result EventPersistenceResult) IsIdempotentSuccess() bool {
	if result.Err != nil {
		return false
	}
	switch result.Status {
	case EventPersistenceInserted,
		EventPersistenceDuplicateExact,
		EventPersistenceDuplicateLogical,
		EventPersistenceDuplicateIdempotency:
		return true
	default:
		return false
	}
}

// EventRepository define el puerto para persistir eventos validos.
//
// Recibe un contexto y una lista de event.ValidatedEvent. Devuelve exactamente
// un EventPersistenceResult por entrada y conserva la atomicidad de cada evento
// con sus detalles. Los errores no clasificables se informan dentro del
// resultado correspondiente para no revertir entradas no relacionadas.
//
// Debe implementarse en adaptadores outbound, inicialmente PostgreSQL y en el
// futuro ClickHouse sin modificar los casos de uso.
type EventRepository interface {
	SaveBatch(ctx context.Context, events []event.ValidatedEvent) []EventPersistenceResult
}
