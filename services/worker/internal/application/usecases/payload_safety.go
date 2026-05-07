package usecases

import (
	"strings"

	"goanalytics/services/worker/internal/application/dto"
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
	if key := blockedKeyInMap(raw.Properties); key != "" {
		return key
	}
	return blockedKeyInMap(raw.Context)
}

func blockedKeyInMap(values map[string]any) string {
	for key, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if _, blocked := blockedPayloadKeys[normalized]; blocked {
			return key
		}
		if nested, ok := value.(map[string]any); ok {
			if found := blockedKeyInMap(nested); found != "" {
				return found
			}
		}
	}
	return ""
}
