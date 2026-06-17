package usecases

import (
	"testing"
	"time"

	"goanalytics/services/worker/internal/domain/event"
)

func TestBuildItemDetailsFromSingleItemProperties(t *testing.T) {
	valid := validatedItemEvent("item_viewed", map[string]any{
		"item_id":       "100",
		"variant_id":    "101",
		"sku":           "SKU-101",
		"item_type":     "product",
		"item_class_id": "20",
		"category_ids":  []any{"1", float64(2)},
		"surface":       "catalog",
		"position":      float64(3),
		"unit_price":    "1200.50",
		"currency":      "ARS",
	})

	details := buildItemDetails(valid)
	if len(details) != 1 {
		t.Fatalf("details = %d, want 1", len(details))
	}
	got := details[0]
	if got.ItemID != "100" || got.VariantID != "101" || got.SKU != "SKU-101" {
		t.Fatalf("ids = %#v, want item/variant/sku", got)
	}
	if got.ItemType != "product" || got.ItemClassID != "20" {
		t.Fatalf("tipo/clase = %q/%q", got.ItemType, got.ItemClassID)
	}
	if len(got.CategoryIDs) != 2 || got.CategoryIDs[0] != "1" || got.CategoryIDs[1] != "2" {
		t.Fatalf("CategoryIDs = %#v, want [1 2]", got.CategoryIDs)
	}
	if got.Position == nil || *got.Position != 3 {
		t.Fatalf("Position = %#v, want 3", got.Position)
	}
	if got.UnitPrice == nil || *got.UnitPrice != 1200.50 {
		t.Fatalf("UnitPrice = %#v, want 1200.50", got.UnitPrice)
	}
}

func TestBuildItemDetailsFromItemsList(t *testing.T) {
	valid := validatedItemEvent("checkout_started", map[string]any{
		"cart_id": "cart_1",
		"items": []any{
			map[string]any{"item_id": "100", "quantity": float64(2), "cart_id": "cart_1"},
			map[string]any{"item_id": "200", "quantity": float64(1), "cart_id": "cart_1"},
		},
	})

	details := buildItemDetails(valid)
	if len(details) != 2 {
		t.Fatalf("details = %d, want 2", len(details))
	}
	if details[0].ItemID != "100" || details[1].ItemID != "200" {
		t.Fatalf("ItemID = %q/%q, want 100/200", details[0].ItemID, details[1].ItemID)
	}
	if details[0].Quantity == nil || *details[0].Quantity != 2 {
		t.Fatalf("Quantity = %#v, want 2", details[0].Quantity)
	}
}

func TestBuildItemDetailsReturnsEmptyForEventWithoutItems(t *testing.T) {
	valid := validatedItemEvent("page_view", map[string]any{"route_name": "home"})

	details := buildItemDetails(valid)
	if len(details) != 0 {
		t.Fatalf("details = %#v, want empty", details)
	}
}

func TestBuildItemDetailsMarksIncompleteImpression(t *testing.T) {
	valid := validatedItemEvent("item_impression", map[string]any{
		"item_id":         "100",
		"surface":         "catalog",
		"visible_ratio":   float64(60),
		"visible_time_ms": float64(1200),
	})

	details := buildItemDetails(valid)
	if len(details) != 1 {
		t.Fatalf("details = %d, want 1", len(details))
	}
	got := details[0]
	if !got.IncompleteForScoring {
		t.Fatalf("IncompleteForScoring = false, want true")
	}
	if len(got.MissingFields) != 1 || got.MissingFields[0] != "list_instance_id" {
		t.Fatalf("MissingFields = %#v, want list_instance_id", got.MissingFields)
	}
}

func TestBuildItemDetailsMarksIncompletePurchaseLine(t *testing.T) {
	valid := validatedItemEvent("purchase_completed", map[string]any{
		"order_id": "ord_1",
		"items": []any{
			map[string]any{"item_id": "100", "order_id": "ord_1"},
		},
	})

	details := buildItemDetails(valid)
	if len(details) != 1 {
		t.Fatalf("details = %d, want 1", len(details))
	}
	if !details[0].IncompleteForScoring {
		t.Fatalf("IncompleteForScoring = false, want true")
	}
}

func TestBuildOrderDetailFromPurchaseProperties(t *testing.T) {
	valid := validatedItemEvent("purchase_completed", map[string]any{
		"order_id":         "ord_1",
		"currency":         "ARS",
		"subtotal_amount":  float64(1000),
		"net_amount":       float64(900),
		"payment_provider": "gateway",
	})

	order := buildOrderDetail(valid)
	if order == nil {
		t.Fatalf("order = nil, want detail")
	}
	if order.OrderID != "ord_1" || order.Currency != "ARS" {
		t.Fatalf("order = %#v, want ord_1/ARS", order)
	}
	if order.NetAmount == nil || *order.NetAmount != 900 {
		t.Fatalf("NetAmount = %#v, want 900", order.NetAmount)
	}
}

func validatedItemEvent(name string, properties map[string]any) event.ValidatedEvent {
	return event.ValidatedEvent{
		EventID:     "evt_1",
		TenantID:    "tenant_123",
		SiteID:      "site_456",
		SiteCode:    "pub_site_abc123",
		EventName:   name,
		EventTime:   time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		ReceivedAt:  time.Date(2026, 6, 17, 12, 0, 1, 0, time.UTC),
		AnonymousID: "anon_1",
		SessionID:   "sess_1",
		Properties:  properties,
		Context:     map[string]any{},
	}
}
