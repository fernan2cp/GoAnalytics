package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const dedupPrefix = "goanalytics:dedup:event:"

// Deduplicator implementa deduplicacion de event_id con Redis.
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

// Seen indica si un event_id ya fue procesado.
//
// Recibe contexto y event_id. Devuelve true si Redis contiene la marca, false
// si no existe y error si Redis falla.
func (dedup *Deduplicator) Seen(ctx context.Context, eventID string) (bool, error) {
	if strings.TrimSpace(eventID) == "" {
		return false, nil
	}
	count, err := dedup.client.Exists(ctx, dedupKey(eventID)).Result()
	return count > 0, err
}

// Mark registra un event_id como procesado.
//
// Recibe contexto y event_id. Devuelve error si Redis rechaza la escritura. La
// marca usa NX para no modificar marcas previas.
func (dedup *Deduplicator) Mark(ctx context.Context, eventID string) error {
	if strings.TrimSpace(eventID) == "" {
		return nil
	}
	return dedup.client.SetNX(ctx, dedupKey(eventID), "1", dedup.ttl).Err()
}

func dedupKey(eventID string) string {
	return dedupPrefix + strings.TrimSpace(eventID)
}
