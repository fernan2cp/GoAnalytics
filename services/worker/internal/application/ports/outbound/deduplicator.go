package outbound

import "context"

// Deduplicator define el puerto para detectar eventos ya procesados.
//
// Seen recibe un event_id y devuelve true si ya fue observado, false si no, y
// error ante fallas de infraestructura. Mark registra un event_id como visto y
// devuelve error si no puede persistir la marca.
//
// Debe implementarse con Redis, base de datos u otro almacenamiento idempotente
// segun la fase. El dominio no debe conocer ese mecanismo.
type Deduplicator interface {
	Seen(ctx context.Context, eventID string) (bool, error)
	Mark(ctx context.Context, eventID string) error
}
