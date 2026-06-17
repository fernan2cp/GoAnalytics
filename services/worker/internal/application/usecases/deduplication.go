package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/site"
)

const (
	dedupStrategyExact          = "exact"
	dedupStrategyLogical        = "logical"
	dedupStrategyIdempotency    = "idempotency"
	dedupStrategySemantic       = "semantic"
	dedupStrategyItemImpression = "item_impression"
	dedupStrategyPurchaseLine   = "purchase_line"
	dedupStrategyNone           = "none"
)

// SemanticDedupRule define una regla explicita de deduplicacion semantica.
//
// EventName indica el evento al que aplica. Window define la ventana temporal
// conservadora usada por Redis. Fields enumera datos permitidos para construir
// la clave estable. Debe usarse solo como respaldo cuando no exista una clave
// logica o idempotente fuerte.
type SemanticDedupRule struct {
	EventName string
	Window    time.Duration
	Fields    []string
}

// dedupCandidate representa una clave candidata para deduplicar un evento.
//
// Contiene la estrategia auditable y la clave lista para consultar en el
// puerto Deduplicator. Empty indica que no hay deduplicacion de esa capa.
type dedupCandidate struct {
	Strategy string
	Key      outbound.DeduplicationKey
	Empty    bool
}

// exactDedupCandidate arma la clave tecnica exacta por event_id.
func exactDedupCandidate(raw dto.RawEvent) dedupCandidate {
	return dedupCandidate{
		Strategy: dedupStrategyExact,
		Key: outbound.DeduplicationKey{
			Strategy: dedupStrategyExact,
			Key:      strings.TrimSpace(raw.EventID),
		},
	}
}

// strongDedupCandidate prioriza claves fuertes enviadas por SDK o dominio.
func strongDedupCandidate(raw dto.RawEvent, config site.SiteConfig) dedupCandidate {
	if key := strings.TrimSpace(raw.IdempotencyKey); key != "" {
		return dedupCandidate{
			Strategy: dedupStrategyIdempotency,
			Key: outbound.DeduplicationKey{
				Strategy: dedupStrategyIdempotency,
				Key:      scopedKey(config, key),
			},
		}
	}
	if key := strings.TrimSpace(raw.LogicalEventID); key != "" {
		return dedupCandidate{
			Strategy: dedupStrategyLogical,
			Key: outbound.DeduplicationKey{
				Strategy: dedupStrategyLogical,
				Key:      scopedKey(config, key),
			},
		}
	}
	return dedupCandidate{Empty: true}
}

// semanticDedupCandidate arma una clave semantica solo si existe regla explicita.
func semanticDedupCandidate(raw dto.RawEvent, config site.SiteConfig, rules []SemanticDedupRule) dedupCandidate {
	if strings.TrimSpace(raw.LogicalEventID) != "" || strings.TrimSpace(raw.IdempotencyKey) != "" {
		return dedupCandidate{Empty: true}
	}
	for _, rule := range rules {
		if strings.TrimSpace(rule.EventName) != raw.EventName || rule.Window <= 0 {
			continue
		}
		parts := make([]string, 0, len(rule.Fields)+2)
		parts = append(parts, config.TenantID, config.SiteID, raw.EventName)
		for _, field := range rule.Fields {
			parts = append(parts, semanticFieldValue(field, raw, config))
		}
		sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
		windowBucket := raw.EventTime.UnixNano() / int64(rule.Window)
		return dedupCandidate{
			Strategy: dedupStrategySemantic,
			Key: outbound.DeduplicationKey{
				Strategy: dedupStrategySemantic,
				Key:      fmt.Sprintf("%s:%d", hex.EncodeToString(sum[:]), windowBucket),
			},
		}
	}
	return dedupCandidate{Empty: true}
}

func scopedKey(config site.SiteConfig, key string) string {
	return strings.Join([]string{config.TenantID, config.SiteID, strings.TrimSpace(key)}, ":")
}

func semanticFieldValue(field string, raw dto.RawEvent, config site.SiteConfig) string {
	switch strings.TrimSpace(field) {
	case "tenant_id":
		return config.TenantID
	case "site_id":
		return config.SiteID
	case "site_code":
		return raw.SiteCode
	case "session_id":
		return raw.SessionID
	case "tab_id":
		return raw.TabID
	case "path":
		return raw.Path
	case "url":
		return raw.URL
	case "event_name":
		return raw.EventName
	case "anonymous_id":
		return raw.AnonymousID
	default:
		return ""
	}
}

// itemSpecificDedupKeys arma claves de deduplicacion basadas en detalles de item.
func itemSpecificDedupKeys(valid event.ValidatedEvent) []outbound.DeduplicationKey {
	switch strings.TrimSpace(valid.EventName) {
	case "item_impression":
		return itemImpressionDedupKeys(valid)
	case "purchase_completed":
		return purchaseLineDedupKeys(valid)
	default:
		return nil
	}
}

func itemImpressionDedupKeys(valid event.ValidatedEvent) []outbound.DeduplicationKey {
	keys := make([]outbound.DeduplicationKey, 0, len(valid.ItemDetails))
	for _, detail := range valid.ItemDetails {
		if strings.TrimSpace(detail.ItemID) == "" ||
			strings.TrimSpace(detail.Surface) == "" ||
			strings.TrimSpace(detail.ListInstanceID) == "" ||
			strings.TrimSpace(detail.SessionID) == "" {
			continue
		}
		key := strings.Join([]string{
			detail.TenantID,
			detail.SiteID,
			detail.SessionID,
			detail.Surface,
			detail.ListInstanceID,
			detail.ItemID,
			detail.VariantID,
		}, ":")
		keys = append(keys, outbound.DeduplicationKey{Strategy: dedupStrategyItemImpression, Key: key})
	}
	return keys
}

func purchaseLineDedupKeys(valid event.ValidatedEvent) []outbound.DeduplicationKey {
	keys := make([]outbound.DeduplicationKey, 0, len(valid.ItemDetails))
	for _, detail := range valid.ItemDetails {
		if strings.TrimSpace(detail.OrderID) == "" || strings.TrimSpace(detail.OrderLineID) == "" {
			continue
		}
		key := strings.Join([]string{
			detail.TenantID,
			detail.SiteID,
			detail.OrderID,
			detail.OrderLineID,
		}, ":")
		keys = append(keys, outbound.DeduplicationKey{Strategy: dedupStrategyPurchaseLine, Key: key})
	}
	return keys
}
