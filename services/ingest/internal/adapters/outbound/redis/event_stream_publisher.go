package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"goanalytics/services/ingest/internal/domain/event"
)

// EventStreamPublisher publica eventos crudos en Redis Stream.
//
// Contiene el cliente Redis, nombre del stream y maxlen aproximado. Implementa
// el puerto EventPublisher para que la aplicacion no conozca go-redis.
type EventStreamPublisher struct {
	client     redis.UniversalClient
	streamName string
	maxLen     int64
}

// NewEventStreamPublisher crea un publisher basado en Redis Stream.
//
// Recibe cliente Redis, nombre del stream y maxlen aproximado. Devuelve error
// si falta una dependencia obligatoria. Maxlen menor o igual a cero deja el
// stream sin recorte automatico.
func NewEventStreamPublisher(client redis.UniversalClient, streamName string, maxLen int64) (*EventStreamPublisher, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente redis requerido")
	}
	if streamName == "" {
		return nil, fmt.Errorf("stream redis requerido")
	}
	return &EventStreamPublisher{client: client, streamName: streamName, maxLen: maxLen}, nil
}

// Publish envia eventos aceptados al stream de Redis.
//
// Recibe contexto y una lista de event.RawEvent enriquecidos. Devuelve error
// cuando falla la serializacion o Redis rechaza la escritura. Cada entrada se
// publica con el evento completo en JSON para mantener compatibilidad futura.
func (publisher *EventStreamPublisher) Publish(ctx context.Context, events []event.RawEvent) error {
	for _, raw := range events {
		payload, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		args := &redis.XAddArgs{
			Stream: publisher.streamName,
			Values: map[string]any{
				"schema_version": 1,
				"event_id":       raw.EventID,
				"site_code":      raw.SiteCode,
				"event_name":     raw.EventName,
				"payload":        string(payload),
			},
		}
		if publisher.maxLen > 0 {
			args.MaxLen = publisher.maxLen
			args.Approx = true
		}
		if err := publisher.client.XAdd(ctx, args).Err(); err != nil {
			return err
		}
	}
	return nil
}
