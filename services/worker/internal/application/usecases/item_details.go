package usecases

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"goanalytics/services/worker/internal/domain/event"
)

// buildItemDetails normaliza datos de items asociados a un evento validado.
//
// Recibe un evento ya validado y devuelve cero, una o muchas filas logicas de
// item. No persiste datos ni aplica reglas SQL; solo traduce properties hacia
// tipos de dominio usados por el puerto de persistencia.
func buildItemDetails(valid event.ValidatedEvent) []event.ItemDetail {
	items := extractItemPayloads(valid.Properties)
	if len(items) == 0 {
		return nil
	}
	details := make([]event.ItemDetail, 0, len(items))
	for _, payload := range items {
		detail := itemDetailFromPayload(valid, payload)
		markItemDetailCompleteness(&detail)
		details = append(details, detail)
	}
	return details
}

// buildOrderDetail normaliza la cabecera de checkout u orden cuando existe.
func buildOrderDetail(valid event.ValidatedEvent) *event.OrderDetail {
	if valid.EventName != "checkout_started" && valid.EventName != "purchase_completed" {
		return nil
	}
	source := valid.Properties
	if nested := mapValue(valid.Properties, "order"); nested != nil {
		source = mergeDetailMaps(valid.Properties, nested)
	}
	return &event.OrderDetail{
		ClientEventID:    valid.EventID,
		TenantID:         valid.TenantID,
		SiteID:           valid.SiteID,
		SiteCode:         valid.SiteCode,
		EventName:        valid.EventName,
		EventTime:        valid.EventTime,
		CartID:           stringValue(source, "cart_id"),
		CheckoutID:       stringValue(source, "checkout_id"),
		OrderID:          stringValue(source, "order_id"),
		Currency:         stringValue(source, "currency"),
		SubtotalAmount:   floatValue(source, "subtotal_amount"),
		DiscountAmount:   floatValue(source, "discount_amount"),
		ShippingAmount:   floatValue(source, "shipping_amount"),
		TaxAmount:        floatValue(source, "tax_amount"),
		GrossAmount:      floatValue(source, "gross_amount"),
		NetAmount:        floatValue(source, "net_amount"),
		CostAmount:       floatValue(source, "cost_amount"),
		PaymentMethodID:  stringValue(source, "payment_method_id"),
		PaymentProvider:  stringValue(source, "payment_provider"),
		ShippingMethodID: stringValue(source, "shipping_method_id"),
		Metadata:         detailMetadata(source),
	}
}

// extractItemPayloads detecta items declarados en distintas formas compatibles.
func extractItemPayloads(properties map[string]any) []map[string]any {
	if len(properties) == 0 {
		return nil
	}
	if items := itemListValue(properties["items"]); len(items) > 0 {
		return items
	}
	if item := mapValue(properties, "item"); item != nil {
		return []map[string]any{mergeDetailMaps(properties, item)}
	}
	if strings.TrimSpace(stringValue(properties, "item_id")) != "" {
		return []map[string]any{properties}
	}
	return nil
}

func itemDetailFromPayload(valid event.ValidatedEvent, payload map[string]any) event.ItemDetail {
	return event.ItemDetail{
		ClientEventID:     valid.EventID,
		LogicalEventID:    valid.LogicalEventID,
		TenantID:          valid.TenantID,
		SiteID:            valid.SiteID,
		SiteCode:          valid.SiteCode,
		EventName:         valid.EventName,
		EventTime:         valid.EventTime,
		ReceivedAt:        valid.ReceivedAt,
		AnonymousID:       valid.AnonymousID,
		SessionID:         valid.SessionID,
		UserID:            valid.UserID,
		ItemID:            stringValue(payload, "item_id"),
		VariantID:         stringValue(payload, "variant_id"),
		SKU:               stringValue(payload, "sku"),
		ItemType:          stringValue(payload, "item_type"),
		ItemClassID:       stringValue(payload, "item_class_id"),
		CategoryIDs:       stringListValue(payload["category_ids"]),
		Surface:           stringValue(payload, "surface"),
		Position:          intValue(payload, "position"),
		Page:              intValue(payload, "page"),
		SearchTerm:        stringValue(payload, "search_term"),
		RankingRunID:      stringValue(payload, "ranking_run_id"),
		RankingVersion:    stringValue(payload, "ranking_version"),
		ListInstanceID:    stringValue(payload, "list_instance_id"),
		ImpressionBatchID: stringValue(payload, "impression_batch_id"),
		VisibleRatio:      floatValue(payload, "visible_ratio"),
		VisibleTimeMs:     int64Value(payload, "visible_time_ms"),
		ViewportWidth:     intValue(payload, "viewport_width"),
		ViewportHeight:    intValue(payload, "viewport_height"),
		RenderedAt:        timeValue(payload, "rendered_at"),
		CartID:            stringValue(payload, "cart_id"),
		CheckoutID:        stringValue(payload, "checkout_id"),
		OrderID:           stringValue(payload, "order_id"),
		OrderLineID:       stringValue(payload, "order_line_id"),
		Quantity:          floatValue(payload, "quantity"),
		UnitPrice:         floatValue(payload, "unit_price"),
		Currency:          stringValue(payload, "currency"),
		GrossAmount:       floatValue(payload, "gross_amount"),
		NetAmount:         floatValue(payload, "net_amount"),
		DiscountAmount:    floatValue(payload, "discount_amount"),
		UnitCost:          floatValue(payload, "unit_cost"),
		CostAmount:        floatValue(payload, "cost_amount"),
		Metadata:          detailMetadata(payload),
	}
}

func markItemDetailCompleteness(detail *event.ItemDetail) {
	var missing []string
	if strings.TrimSpace(detail.ItemID) == "" {
		missing = append(missing, "item_id")
	}
	switch detail.EventName {
	case "item_impression":
		if strings.TrimSpace(detail.Surface) == "" {
			missing = append(missing, "surface")
		}
		if strings.TrimSpace(detail.ListInstanceID) == "" {
			missing = append(missing, "list_instance_id")
		}
		if detail.VisibleRatio == nil {
			missing = append(missing, "visible_ratio")
		}
		if detail.VisibleTimeMs == nil {
			missing = append(missing, "visible_time_ms")
		}
	case "purchase_completed":
		if strings.TrimSpace(detail.OrderID) == "" {
			missing = append(missing, "order_id")
		}
		if strings.TrimSpace(detail.OrderLineID) == "" {
			missing = append(missing, "order_line_id")
		}
	}
	detail.MissingFields = missing
	detail.IncompleteForScoring = len(missing) > 0
	if len(missing) > 0 {
		if detail.Metadata == nil {
			detail.Metadata = map[string]any{}
		}
		detail.Metadata["missing_fields"] = append([]string(nil), missing...)
		detail.Metadata["incomplete_for_scoring"] = true
	}
}

func itemListValue(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, raw := range typed {
			if item, ok := raw.(map[string]any); ok {
				items = append(items, item)
			}
		}
		return items
	default:
		return nil
	}
}

func mapValue(values map[string]any, key string) map[string]any {
	value, ok := values[key]
	if !ok {
		return nil
	}
	nested, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return nested
}

func mergeDetailMaps(primary map[string]any, secondary map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range primary {
		merged[key] = value
	}
	for key, value := range secondary {
		merged[key] = value
	}
	return merged
}

func detailMetadata(payload map[string]any) map[string]any {
	if metadata := mapValue(payload, "metadata"); metadata != nil {
		return metadata
	}
	return map[string]any{}
}

func stringValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case float64:
		if math.Trunc(typed) == typed {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func stringListValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return normalizedStrings(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				values = append(values, text)
			}
		}
		return values
	case string:
		return normalizedStrings(strings.Split(typed, ","))
	default:
		return nil
	}
}

func normalizedStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" {
			normalized = append(normalized, text)
		}
	}
	return normalized
}

func intValue(values map[string]any, key string) *int {
	if value := int64Value(values, key); value != nil {
		converted := int(*value)
		return &converted
	}
	return nil
}

func int64Value(values map[string]any, key string) *int64 {
	number := floatValue(values, key)
	if number == nil {
		return nil
	}
	converted := int64(*number)
	return &converted
}

func floatValue(values map[string]any, key string) *float64 {
	value, ok := values[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		return &typed
	case float32:
		converted := float64(typed)
		return &converted
	case int:
		converted := float64(typed)
		return &converted
	case int64:
		converted := float64(typed)
		return &converted
	case jsonNumber:
		if parsed, err := strconv.ParseFloat(string(typed), 64); err == nil {
			return &parsed
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return &parsed
		}
	}
	return nil
}

func timeValue(values map[string]any, key string) *time.Time {
	value, ok := values[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case time.Time:
		return &typed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return &parsed
		}
	}
	return nil
}

type jsonNumber string
