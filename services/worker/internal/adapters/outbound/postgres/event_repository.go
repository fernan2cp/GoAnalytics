package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/event"
)

// EventRepository persiste eventos validos en PostgreSQL.
//
// Contiene un pool pgxpool y encapsula SQL, transacciones y conversion JSONB.
// Implementa el puerto EventRepository del worker sin exponer detalles de base
// de datos al nucleo de aplicacion.
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

// SaveBatch inserta eventos validos y clasifica cada resultado.
//
// Recibe contexto y eventos de dominio. Devuelve un resultado por entrada y
// usa una transaccion independiente para cada evento base con sus detalles.
// Los conflictos unicos esperables se clasifican; una falla no clasificable se
// conserva en el resultado afectado sin revertir otros eventos.
func (repository *EventRepository) SaveBatch(ctx context.Context, events []event.ValidatedEvent) []outbound.EventPersistenceResult {
	results := make([]outbound.EventPersistenceResult, 0, len(events))
	for _, item := range events {
		result := outbound.EventPersistenceResult{EventID: item.EventID}
		status, existingEventID, conflictField, err := repository.saveOne(ctx, item)
		result.Status = status
		result.ExistingEventID = existingEventID
		result.ConflictField = conflictField
		result.Err = err
		results = append(results, result)
	}
	return results
}

// saveOne persiste un evento y sus detalles en una transaccion aislada.
//
// Recibe contexto y evento validado. Devuelve estado, ID preexistente, campo
// conflictivo y error. Un duplicado no modifica la fila ni sus detalles.
func (repository *EventRepository) saveOne(ctx context.Context, item event.ValidatedEvent) (outbound.EventPersistenceStatus, string, string, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return "", "", "", err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	eventRowID, inserted, err := saveValidatedEvent(ctx, tx, item)
	if err != nil {
		return "", "", "", err
	}
	if !inserted {
		status, existingEventID, conflictField, err := classifyEventConflict(ctx, tx, item)
		return status, existingEventID, conflictField, err
	}
	if err := replaceItemDetails(ctx, tx, eventRowID, item.ItemDetails); err != nil {
		return "", "", "", err
	}
	if err := replaceOrderDetail(ctx, tx, eventRowID, item.OrderDetail); err != nil {
		return "", "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", "", err
	}
	return outbound.EventPersistenceInserted, "", "", nil
}

// saveValidatedEvent intenta insertar la fila base sin sobrescribir duplicados.
//
// Recibe transaccion y evento. Devuelve ID, indicador de insercion y error de
// serializacion o PostgreSQL. ErrNoRows representa un conflicto esperado.
func saveValidatedEvent(ctx context.Context, tx pgx.Tx, item event.ValidatedEvent) (int64, bool, error) {
	properties, err := json.Marshal(nonNilMap(item.Properties))
	if err != nil {
		return 0, false, err
	}
	contextPayload, err := json.Marshal(nonNilMap(item.Context))
	if err != nil {
		return 0, false, err
	}
	var eventRowID int64
	err = tx.QueryRow(ctx, insertEventSQL,
		item.EventID,
		emptyToNil(item.LogicalEventID),
		emptyToNil(item.IdempotencyKey),
		emptyToNil(item.TabID),
		zeroToNil(item.Sequence),
		emptyToNil(item.PreviousLogicalEventID),
		emptyToNil(item.DedupStrategy),
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
	).Scan(&eventRowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return eventRowID, true, nil
}

// classifyEventConflict identifica y compara la fila que impidio insertar.
//
// Recibe transaccion y evento entrante. Devuelve duplicate_exact,
// duplicate_logical, duplicate_idempotency o conflict. La prioridad es
// event_id, logical_event_id e idempotency_key.
func classifyEventConflict(ctx context.Context, tx pgx.Tx, item event.ValidatedEvent) (outbound.EventPersistenceStatus, string, string, error) {
	existing, conflictField, err := loadConflictingEvent(ctx, tx, item)
	if err != nil {
		return "", "", "", err
	}
	incomingFingerprint, err := persistenceFingerprintFromEvent(item)
	if err != nil {
		return "", "", "", err
	}
	existingFingerprint, err := persistenceFingerprintFromStored(existing)
	if err != nil {
		return "", "", "", err
	}
	if incomingFingerprint != existingFingerprint {
		return outbound.EventPersistenceConflict, existing.EventID, conflictField, nil
	}
	switch conflictField {
	case "event_id":
		return outbound.EventPersistenceDuplicateExact, existing.EventID, conflictField, nil
	case "logical_event_id":
		return outbound.EventPersistenceDuplicateLogical, existing.EventID, conflictField, nil
	case "idempotency_key":
		return outbound.EventPersistenceDuplicateIdempotency, existing.EventID, conflictField, nil
	default:
		return "", "", "", fmt.Errorf("campo de conflicto desconocido: %s", conflictField)
	}
}

type storedEvent struct {
	EventID                string
	LogicalEventID         string
	IdempotencyKey         string
	TabID                  string
	Sequence               int64
	PreviousLogicalEventID string
	TenantID               string
	SiteID                 string
	SiteCode               string
	Environment            string
	EventName              string
	EventVersion           int
	EventTime              time.Time
	AnonymousID            string
	UserID                 *string
	SessionID              string
	Origin                 string
	URL                    string
	Path                   string
	Properties             map[string]any
	Context                map[string]any
}

// loadConflictingEvent consulta la fila conflictiva segun prioridad estable.
//
// Recibe transaccion y evento entrante. Devuelve la fila, el campo que
// coincidio o error si PostgreSQL no encuentra la causa del conflicto.
func loadConflictingEvent(ctx context.Context, tx pgx.Tx, item event.ValidatedEvent) (storedEvent, string, error) {
	var existing storedEvent
	var logicalEventID *string
	var idempotencyKey *string
	var tabID *string
	var sequence *int64
	var previousLogicalEventID *string
	var anonymousID *string
	var sessionID *string
	var origin *string
	var url *string
	var path *string
	var propertiesJSON []byte
	var contextJSON []byte
	var conflictField string
	err := tx.QueryRow(ctx, selectConflictingEventSQL,
		item.EventID,
		item.TenantID,
		item.SiteID,
		emptyToNil(item.LogicalEventID),
		emptyToNil(item.IdempotencyKey),
	).Scan(
		&existing.EventID,
		&logicalEventID,
		&idempotencyKey,
		&tabID,
		&sequence,
		&previousLogicalEventID,
		&existing.TenantID,
		&existing.SiteID,
		&existing.SiteCode,
		&existing.Environment,
		&existing.EventName,
		&existing.EventVersion,
		&existing.EventTime,
		&anonymousID,
		&existing.UserID,
		&sessionID,
		&origin,
		&url,
		&path,
		&propertiesJSON,
		&contextJSON,
		&conflictField,
	)
	if err != nil {
		return storedEvent{}, "", err
	}
	existing.LogicalEventID = stringOrEmpty(logicalEventID)
	existing.IdempotencyKey = stringOrEmpty(idempotencyKey)
	existing.TabID = stringOrEmpty(tabID)
	if sequence != nil {
		existing.Sequence = *sequence
	}
	existing.PreviousLogicalEventID = stringOrEmpty(previousLogicalEventID)
	existing.AnonymousID = stringOrEmpty(anonymousID)
	existing.SessionID = stringOrEmpty(sessionID)
	existing.Origin = stringOrEmpty(origin)
	existing.URL = stringOrEmpty(url)
	existing.Path = stringOrEmpty(path)
	if err := json.Unmarshal(propertiesJSON, &existing.Properties); err != nil {
		return storedEvent{}, "", err
	}
	if err := json.Unmarshal(contextJSON, &existing.Context); err != nil {
		return storedEvent{}, "", err
	}
	return existing, conflictField, nil
}

type persistenceFingerprint struct {
	LogicalEventID         string         `json:"logical_event_id"`
	IdempotencyKey         string         `json:"idempotency_key"`
	TabID                  string         `json:"tab_id"`
	Sequence               int64          `json:"sequence"`
	PreviousLogicalEventID string         `json:"previous_logical_event_id"`
	TenantID               string         `json:"tenant_id"`
	SiteID                 string         `json:"site_id"`
	SiteCode               string         `json:"site_code"`
	Environment            string         `json:"env"`
	EventName              string         `json:"event_name"`
	EventVersion           int            `json:"event_version"`
	EventTime              time.Time      `json:"event_time"`
	AnonymousID            string         `json:"anonymous_id"`
	UserID                 *string        `json:"user_id"`
	SessionID              string         `json:"session_id"`
	Origin                 string         `json:"origin"`
	URL                    string         `json:"url"`
	Path                   string         `json:"path"`
	Properties             map[string]any `json:"properties"`
	Context                map[string]any `json:"context"`
}

// persistenceFingerprintFromEvent genera la huella semantica entrante.
//
// Recibe evento validado y devuelve JSON canonico o error de serializacion. No
// incluye IDs de reintento ni metadata de transporte mutable.
func persistenceFingerprintFromEvent(item event.ValidatedEvent) (string, error) {
	return marshalPersistenceFingerprint(persistenceFingerprint{
		LogicalEventID: item.LogicalEventID, IdempotencyKey: item.IdempotencyKey,
		TabID: item.TabID, Sequence: item.Sequence, PreviousLogicalEventID: item.PreviousLogicalEventID,
		TenantID: item.TenantID, SiteID: item.SiteID, SiteCode: item.SiteCode,
		Environment: item.Environment, EventName: item.EventName, EventVersion: item.EventVersion,
		EventTime: normalizePostgresTime(item.EventTime), AnonymousID: item.AnonymousID,
		UserID: item.UserID, SessionID: item.SessionID, Origin: item.Origin, URL: item.URL, Path: item.Path,
		Properties: nonNilMap(item.Properties), Context: nonNilMap(item.Context),
	})
}

// persistenceFingerprintFromStored genera la huella semantica persistida.
//
// Recibe fila existente y devuelve JSON canonico o error de serializacion.
func persistenceFingerprintFromStored(item storedEvent) (string, error) {
	return marshalPersistenceFingerprint(persistenceFingerprint{
		LogicalEventID: item.LogicalEventID, IdempotencyKey: item.IdempotencyKey,
		TabID: item.TabID, Sequence: item.Sequence, PreviousLogicalEventID: item.PreviousLogicalEventID,
		TenantID: item.TenantID, SiteID: item.SiteID, SiteCode: item.SiteCode,
		Environment: item.Environment, EventName: item.EventName, EventVersion: item.EventVersion,
		EventTime: normalizePostgresTime(item.EventTime), AnonymousID: item.AnonymousID,
		UserID: item.UserID, SessionID: item.SessionID, Origin: item.Origin, URL: item.URL, Path: item.Path,
		Properties: nonNilMap(item.Properties), Context: nonNilMap(item.Context),
	})
}

// marshalPersistenceFingerprint serializa una huella con orden JSON estable.
func marshalPersistenceFingerprint(value persistenceFingerprint) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// normalizePostgresTime ajusta timestamps a precision de microsegundos UTC.
func normalizePostgresTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

// stringOrEmpty normaliza un texto nullable leido desde PostgreSQL.
func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func replaceItemDetails(ctx context.Context, tx pgx.Tx, eventRowID int64, details []event.ItemDetail) error {
	if _, err := tx.Exec(ctx, deleteItemDetailsSQL, eventRowID); err != nil {
		return err
	}
	for _, detail := range details {
		metadata, err := json.Marshal(nonNilMap(detail.Metadata))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, insertItemDetailSQL,
			eventRowID,
			detail.ClientEventID,
			emptyToNil(detail.LogicalEventID),
			detail.TenantID,
			detail.SiteID,
			detail.SiteCode,
			detail.EventName,
			detail.EventTime,
			detail.ReceivedAt,
			emptyToNil(detail.AnonymousID),
			emptyToNil(detail.SessionID),
			detail.UserID,
			detail.ItemID,
			emptyToNil(detail.VariantID),
			emptyToNil(detail.SKU),
			emptyToNil(detail.ItemType),
			emptyToNil(detail.ItemClassID),
			detail.CategoryIDs,
			emptyToNil(detail.Surface),
			detail.Position,
			detail.Page,
			emptyToNil(detail.SearchTerm),
			emptyToNil(detail.RankingRunID),
			emptyToNil(detail.RankingVersion),
			emptyToNil(detail.ListInstanceID),
			emptyToNil(detail.ImpressionBatchID),
			detail.VisibleRatio,
			detail.VisibleTimeMs,
			detail.ViewportWidth,
			detail.ViewportHeight,
			detail.RenderedAt,
			emptyToNil(detail.CartID),
			emptyToNil(detail.CheckoutID),
			emptyToNil(detail.OrderID),
			emptyToNil(detail.OrderLineID),
			detail.Quantity,
			detail.UnitPrice,
			emptyToNil(detail.Currency),
			detail.GrossAmount,
			detail.NetAmount,
			detail.DiscountAmount,
			detail.UnitCost,
			detail.CostAmount,
			string(metadata),
		); err != nil {
			return err
		}
	}
	return nil
}

func replaceOrderDetail(ctx context.Context, tx pgx.Tx, eventRowID int64, detail *event.OrderDetail) error {
	if _, err := tx.Exec(ctx, deleteOrderDetailsSQL, eventRowID); err != nil {
		return err
	}
	if detail == nil {
		return nil
	}
	metadata, err := json.Marshal(nonNilMap(detail.Metadata))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, insertOrderDetailSQL,
		eventRowID,
		detail.ClientEventID,
		detail.TenantID,
		detail.SiteID,
		detail.SiteCode,
		detail.EventName,
		detail.EventTime,
		emptyToNil(detail.CartID),
		emptyToNil(detail.CheckoutID),
		emptyToNil(detail.OrderID),
		emptyToNil(detail.Currency),
		detail.SubtotalAmount,
		detail.DiscountAmount,
		detail.ShippingAmount,
		detail.TaxAmount,
		detail.GrossAmount,
		detail.NetAmount,
		detail.CostAmount,
		emptyToNil(detail.PaymentMethodID),
		emptyToNil(detail.PaymentProvider),
		emptyToNil(detail.ShippingMethodID),
		string(metadata),
	)
	return err
}

const insertEventSQL = `
INSERT INTO analytics_events (
	event_id, logical_event_id, idempotency_key, tab_id, sequence,
	previous_logical_event_id, dedup_strategy, tenant_id, site_id, site_code,
	env, event_name, event_version, event_time, received_at, anonymous_id,
	user_id, session_id, origin, url, path, referrer, user_agent, ip_hash,
	sdk_name, sdk_version, jwt_id, token_version, properties, context
) VALUES (
	$1, $2, $3, $4, $5,
	$6, $7, $8, $9, $10,
	$11, $12, $13, $14, $15, $16,
	$17, $18, $19, $20, $21, $22, $23, $24,
	$25, $26, $27, $28, $29::jsonb, $30::jsonb
)
ON CONFLICT DO NOTHING
RETURNING id`

const selectConflictingEventSQL = `
SELECT
	event_id, logical_event_id, idempotency_key, tab_id, sequence,
	previous_logical_event_id, tenant_id, site_id, site_code,
	env, event_name, event_version, event_time, anonymous_id, user_id,
	session_id, origin, url, path, properties, context,
	CASE
		WHEN event_id = $1 THEN 'event_id'
		WHEN $4::text IS NOT NULL AND tenant_id = $2 AND site_id = $3 AND logical_event_id = $4 THEN 'logical_event_id'
		WHEN $5::text IS NOT NULL AND tenant_id = $2 AND site_id = $3 AND idempotency_key = $5 THEN 'idempotency_key'
	END AS conflict_field
FROM analytics_events
WHERE event_id = $1
	OR ($4::text IS NOT NULL AND tenant_id = $2 AND site_id = $3 AND logical_event_id = $4)
	OR ($5::text IS NOT NULL AND tenant_id = $2 AND site_id = $3 AND idempotency_key = $5)
ORDER BY CASE
	WHEN event_id = $1 THEN 1
	WHEN $4::text IS NOT NULL AND tenant_id = $2 AND site_id = $3 AND logical_event_id = $4 THEN 2
	ELSE 3
END
LIMIT 1`

const deleteItemDetailsSQL = `
DELETE FROM analytics_event_items
WHERE analytics_event_id = $1`

const insertItemDetailSQL = `
INSERT INTO analytics_event_items (
	analytics_event_id, client_event_id, logical_event_id, tenant_id, site_id,
	site_code, event_name, event_time, received_at, anonymous_id, session_id,
	user_id, item_id, variant_id, sku, item_type, item_class_id, category_ids,
	surface, position, page, search_term, ranking_run_id, ranking_version,
	list_instance_id, impression_batch_id, visible_ratio, visible_time_ms,
	viewport_width, viewport_height, rendered_at, cart_id, checkout_id,
	order_id, order_line_id, quantity, unit_price, currency, gross_amount,
	net_amount, discount_amount, unit_cost, cost_amount, metadata
) VALUES (
	$1, $2, $3, $4, $5,
	$6, $7, $8, $9, $10, $11,
	$12, $13, $14, $15, $16, $17, $18,
	$19, $20, $21, $22, $23, $24,
	$25, $26, $27, $28,
	$29, $30, $31, $32, $33,
	$34, $35, $36, $37, $38, $39,
	$40, $41, $42, $43, $44::jsonb
)`

const deleteOrderDetailsSQL = `
DELETE FROM analytics_event_orders
WHERE analytics_event_id = $1`

const insertOrderDetailSQL = `
INSERT INTO analytics_event_orders (
	analytics_event_id, client_event_id, tenant_id, site_id, site_code,
	event_name, event_time, cart_id, checkout_id, order_id, currency,
	subtotal_amount, discount_amount, shipping_amount, tax_amount, gross_amount,
	net_amount, cost_amount, payment_method_id, payment_provider,
	shipping_method_id, metadata
) VALUES (
	$1, $2, $3, $4, $5,
	$6, $7, $8, $9, $10, $11,
	$12, $13, $14, $15, $16,
	$17, $18, $19, $20,
	$21, $22::jsonb
)`

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func zeroToNil(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
