package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

const dedupPrefix = "goanalytics:dedup:"

// Deduplicator implementa deduplicacion por estrategia con Redis.
//
// Contiene cliente Redis y TTL opcional para marcas de eventos procesados.
// Implementa el puerto Deduplicator usado por el worker.
type Deduplicator struct {
	client goredis.UniversalClient
	ttl    time.Duration
}

// NewDeduplicator crea un deduplicador Redis.
//
// Recibe cliente Redis y TTL de la marca. Devuelve error si falta el cliente.
// Un TTL no positivo deja la marca sin expiracion.
func NewDeduplicator(client goredis.UniversalClient, ttl time.Duration) (*Deduplicator, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente redis requerido")
	}
	return &Deduplicator{client: client, ttl: ttl}, nil
}

// Seen indica si una clave de deduplicacion ya fue procesada.
//
// Recibe contexto y clave. Devuelve true si Redis contiene la marca, false si
// no existe y error si Redis falla.
func (dedup *Deduplicator) Seen(ctx context.Context, key outbound.DeduplicationKey) (bool, error) {
	if strings.TrimSpace(key.Key) == "" || strings.TrimSpace(key.Strategy) == "" {
		return false, nil
	}
	count, err := dedup.client.Exists(ctx, dedupKey(key)).Result()
	return count > 0, err
}

// Mark registra una clave de deduplicacion como procesada.
//
// Recibe contexto y clave. Devuelve error si Redis rechaza la escritura. La
// marca usa NX para no modificar marcas previas.
func (dedup *Deduplicator) Mark(ctx context.Context, key outbound.DeduplicationKey) error {
	if strings.TrimSpace(key.Key) == "" || strings.TrimSpace(key.Strategy) == "" {
		return nil
	}
	return dedup.client.SetNX(ctx, dedupKey(key), "1", dedup.ttl).Err()
}

func dedupKey(key outbound.DeduplicationKey) string {
	return dedupPrefix + strings.TrimSpace(key.Strategy) + ":" + strings.TrimSpace(key.Key)
}
