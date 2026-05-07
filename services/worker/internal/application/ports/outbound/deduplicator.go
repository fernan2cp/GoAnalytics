package outbound

import "context"

// DeduplicationKey identifica una marca de deduplicacion.
//
// Strategy indica la capa aplicada, por ejemplo exact, logical, idempotency o
// semantic. Key contiene la clave ya normalizada para esa estrategia.
type DeduplicationKey struct {
	Strategy string
	Key      string
}

// Deduplicator define el puerto para detectar eventos ya procesados.
//
// Seen recibe una clave de deduplicacion y devuelve true si ya fue observada,
// false si no, y error ante fallas de infraestructura. Mark registra una clave
// como vista y devuelve error si no puede persistir la marca.
//
// Debe implementarse con Redis, base de datos u otro almacenamiento idempotente
// segun la fase. El dominio no debe conocer ese mecanismo.
type Deduplicator interface {
	Seen(ctx context.Context, key DeduplicationKey) (bool, error)
	Mark(ctx context.Context, key DeduplicationKey) error
}
