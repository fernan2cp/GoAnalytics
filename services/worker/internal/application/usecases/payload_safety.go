package usecases

import (
	"encoding/json"
	"fmt"
	"strings"

	"goanalytics/services/worker/internal/application/dto"
)

const (
	maxPayloadObjectBytes = 64 * 1024
	maxPayloadDepth       = 16
)

var blockedPayloadKeys = map[string]struct{}{
	"password":      {},
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"authorization": {},
	"cookie":        {},
	"secret":        {},
	"private_key":   {},
	"credit_card":   {},
	"card_number":   {},
	"cvv":           {},
	"dni":           {},
	"document":      {},
}

// blockedPayloadKey busca claves sensibles en properties y context.
//
// Recibe un evento crudo y devuelve la primera clave bloqueada encontrada. No
// inspecciona valores para evitar convertir analytics en un scanner de datos
// personales; solo aplica la lista tecnica documentada.
func blockedPayloadKey(raw dto.RawEvent) string {
	if key := blockedKeyInValue(raw.Properties); key != "" {
		return key
	}
	return blockedKeyInValue(raw.Context)
}

func blockedKeyInValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if _, blocked := blockedPayloadKeys[normalized]; blocked {
				return key
			}
			if found := blockedKeyInValue(nested); found != "" {
				return found
			}
		}
	case []any:
		for _, nested := range typed {
			if found := blockedKeyInValue(nested); found != "" {
				return found
			}
		}
	}
	return ""
}

// payloadLimitViolation valida limites estructurales de properties y context.
//
// Devuelve una descripcion estable si el payload supera profundidad o tamanio
// maximo. Los limites aplican al worker porque los eventos tambien pueden
// llegar desde streams internos, no solo desde el handler HTTP de ingesta.
func payloadLimitViolation(raw dto.RawEvent) string {
	if violation := mapLimitViolation("properties", raw.Properties); violation != "" {
		return violation
	}
	return mapLimitViolation("context", raw.Context)
}

func mapLimitViolation(name string, values map[string]any) string {
	if values == nil {
		return ""
	}
	if depth := payloadDepth(values); depth > maxPayloadDepth {
		return fmt.Sprintf("%s_depth", name)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Sprintf("%s_invalid_json", name)
	}
	if len(encoded) > maxPayloadObjectBytes {
		return fmt.Sprintf("%s_size", name)
	}
	return ""
}

func payloadDepth(value any) int {
	switch typed := value.(type) {
	case map[string]any:
		max := 1
		for _, nested := range typed {
			if depth := 1 + payloadDepth(nested); depth > max {
				max = depth
			}
		}
		return max
	case []any:
		max := 1
		for _, nested := range typed {
			if depth := 1 + payloadDepth(nested); depth > max {
				max = depth
			}
		}
		return max
	default:
		return 1
	}
}
