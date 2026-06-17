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

// SaveBatch inserta eventos validos y sus detalles analiticos.
//
// Recibe contexto y eventos de dominio. Usa una transaccion para guardar cada
// evento base junto con sus filas normalizadas de items y cabecera opcional de
// orden. Devuelve error ante fallas de serializacion o PostgreSQL.
func (repository *EventRepository) SaveBatch(ctx context.Context, events []event.ValidatedEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, item := range events {
		eventRowID, err := saveValidatedEvent(ctx, tx, item)
		if err != nil {
			return err
		}
		if err := replaceItemDetails(ctx, tx, eventRowID, item.ItemDetails); err != nil {
			return err
		}
		if err := replaceOrderDetail(ctx, tx, eventRowID, item.OrderDetail); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func saveValidatedEvent(ctx context.Context, tx pgx.Tx, item event.ValidatedEvent) (int64, error) {
	properties, err := json.Marshal(nonNilMap(item.Properties))
	if err != nil {
		return 0, err
	}
	contextPayload, err := json.Marshal(nonNilMap(item.Context))
	if err != nil {
		return 0, err
	}
	var eventRowID int64
	err = tx.QueryRow(ctx, upsertEventSQL,
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
	if err != nil {
		return 0, err
	}
	return eventRowID, nil
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

const upsertEventSQL = `
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
ON CONFLICT (event_id) DO UPDATE SET event_id = EXCLUDED.event_id
RETURNING id`

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
