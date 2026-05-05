package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/domain/rejection"
)

// RejectedEventRepository persiste eventos rechazados en PostgreSQL.
//
// Contiene un pool pgxpool y encapsula SQL, batches y conversion JSONB para la
// tabla analytics_rejected_events.
type RejectedEventRepository struct {
	pool *pgxpool.Pool
}

// NewRejectedEventRepository crea un repositorio PostgreSQL de rechazos.
//
// Recibe un pool ya inicializado por bootstrap. Devuelve error si falta el
// pool. No ejecuta migraciones ni valida esquema.
func NewRejectedEventRepository(pool *pgxpool.Pool) (*RejectedEventRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool postgres requerido")
	}
	return &RejectedEventRepository{pool: pool}, nil
}

// SaveBatch inserta eventos rechazados en analytics_rejected_events.
//
// Recibe contexto y rechazos de dominio. Usa pgx.Batch para insercion por
// batches. Devuelve error ante fallas de serializacion o PostgreSQL.
func (repository *RejectedEventRepository) SaveBatch(ctx context.Context, events []rejection.RejectedEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, item := range events {
		rawPayload, err := json.Marshal(nonNilMap(item.RawPayload))
		if err != nil {
			return err
		}
		batch.Queue(insertRejectedEventSQL,
			item.EventID,
			item.SiteCode,
			item.Environment,
			item.Reason,
			item.Severity,
			item.Origin,
			item.URL,
			item.IPHash,
			item.UserAgent,
			string(rawPayload),
			item.CreatedAt,
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

const insertRejectedEventSQL = `
INSERT INTO analytics_rejected_events (
	event_id, site_code, env, reason, severity, origin, url, ip_hash,
	user_agent, raw_payload, created_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8,
	$9, $10::jsonb, $11
)`
