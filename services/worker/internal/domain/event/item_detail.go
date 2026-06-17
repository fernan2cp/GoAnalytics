package event

import "time"

// ItemDetail representa una fila analitica normalizada asociada a un item.
//
// Contiene datos extraidos desde properties, metadata o items del evento
// publico. Se usa para persistir detalles 1:N de items vinculados a un
// ValidatedEvent sin que application conozca SQL ni tablas concretas.
//
// Los importes y costos son snapshots opcionales del momento del evento. Los
// campos MissingFields e IncompleteForScoring permiten conservar auditoria aun
// cuando falten datos criticos para scoring confiable.
type ItemDetail struct {
	ClientEventID        string
	LogicalEventID       string
	TenantID             string
	SiteID               string
	SiteCode             string
	EventName            string
	EventTime            time.Time
	ReceivedAt           time.Time
	AnonymousID          string
	SessionID            string
	UserID               *string
	ItemID               string
	VariantID            string
	SKU                  string
	ItemType             string
	ItemClassID          string
	CategoryIDs          []string
	Surface              string
	Position             *int
	Page                 *int
	SearchTerm           string
	RankingRunID         string
	RankingVersion       string
	ListInstanceID       string
	ImpressionBatchID    string
	VisibleRatio         *float64
	VisibleTimeMs        *int64
	ViewportWidth        *int
	ViewportHeight       *int
	RenderedAt           *time.Time
	CartID               string
	CheckoutID           string
	OrderID              string
	OrderLineID          string
	Quantity             *float64
	UnitPrice            *float64
	Currency             string
	GrossAmount          *float64
	NetAmount            *float64
	DiscountAmount       *float64
	UnitCost             *float64
	CostAmount           *float64
	Metadata             map[string]any
	MissingFields        []string
	IncompleteForScoring bool
}

// OrderDetail representa la cabecera normalizada de checkout u orden.
//
// Contiene importes y referencias generales del evento cuando este describe
// una compra o inicio de checkout. Es opcional para la primera persistencia,
// pero queda modelado para evitar mezclar cabecera economica con lineas de
// item.
type OrderDetail struct {
	ClientEventID    string
	TenantID         string
	SiteID           string
	SiteCode         string
	EventName        string
	EventTime        time.Time
	CartID           string
	CheckoutID       string
	OrderID          string
	Currency         string
	SubtotalAmount   *float64
	DiscountAmount   *float64
	ShippingAmount   *float64
	TaxAmount        *float64
	GrossAmount      *float64
	NetAmount        *float64
	CostAmount       *float64
	PaymentMethodID  string
	PaymentProvider  string
	ShippingMethodID string
	Metadata         map[string]any
}
