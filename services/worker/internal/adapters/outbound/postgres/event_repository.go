package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/domain/event"
)

// EventRepository persiste eventos validos en PostgreSQL.
//
// Contiene un pool pgxpool y encapsula SQL, batches y conversion JSONB.
// Implementa el puerto EventRepository del worker.
type EventRepository struct {
	pool *pgxpool.Pool
}

// NewEventRepository crea un repositorio PostgreSQL de eventos validos.
//
// Recibe un pool ya inicializado por bootstrap. Devuelve error si falta el
// pool. No ejecuta migraciones ni valida esquema.
func NewEventRepository(pool *pgxpool.Pool) (*EventRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool postgres requerido")
	}
	return &EventRepository{pool: pool}, nil
}

// SaveBatch inserta eventos validos en analytics_events.
//
// Recibe contexto y eventos de dominio. Usa pgx.Batch para insercion por
// batches y ON CONFLICT para que la persistencia sea idempotente por event_id.
// Devuelve error ante fallas de serializacion o PostgreSQL.
func (repository *EventRepository) SaveBatch(ctx context.Context, events []event.ValidatedEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, item := range events {
		properties, err := json.Marshal(nonNilMap(item.Properties))
		if err != nil {
			return err
		}
		contextPayload, err := json.Marshal(nonNilMap(item.Context))
		if err != nil {
			return err
		}
		batch.Queue(insertEventSQL,
			item.EventID,
			item.TenantID,
			item.SiteID,
			item.SiteCode,
			item.Environment,
			item.EventName,
			item.EventVersion,
			item.EventTime,
			item.ReceivedAt,
			item.AnonymousID,
			item.UserID,
			item.SessionID,
			item.Origin,
			item.URL,
			item.Path,
			item.Referrer,
			item.UserAgent,
			item.IPHash,
			item.SDKName,
			item.SDKVersion,
			item.JWTID,
			item.TokenVersion,
			string(properties),
			string(contextPayload),
		)
	}
	results := repository.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range events {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

const insertEventSQL = `
INSERT INTO analytics_events (
	event_id, tenant_id, site_id, site_code, env, event_name, event_version,
	event_time, received_at, anonymous_id, user_id, session_id, origin, url,
	path, referrer, user_agent, ip_hash, sdk_name, sdk_version, jwt_id,
	token_version, properties, context
) VALUES (
	$1, $2, $3, $4, $5, $6, $7,
	$8, $9, $10, $11, $12, $13, $14,
	$15, $16, $17, $18, $19, $20, $21,
	$22, $23::jsonb, $24::jsonb
)
ON CONFLICT (event_id) DO NOTHING`

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
