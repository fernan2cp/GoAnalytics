package event

import (
	"errors"
	"strings"
)

// ErrInvalidRawEvent indica que un evento crudo del worker no es procesable.
//
// Se usa para distinguir rechazos de payload frente a errores de infraestructura
// al consumir, validar o persistir eventos.
var ErrInvalidRawEvent = errors.New("evento crudo invalido")

// ValidateRequiredFields comprueba los campos minimos de un evento crudo.
//
// Recibe los campos ya mapeados desde dto.RawEvent y valida identificadores,
// site_code, entorno, token, nombre, version, tiempos, identidad, origin, URL y
// path. Devuelve ErrInvalidRawEvent cuando falta algun dato obligatorio.
//
// Debe ejecutarse antes de validar metadata de site o deduplicacion. No valida
// reglas de dominio del site ni accede a infraestructura.
func ValidateRequiredFields(
	eventID string,
	siteCode string,
	environment string,
	tokenVersion int,
	jwtID string,
	eventName string,
	eventVersion int,
	eventTimeIsZero bool,
	anonymousID string,
	sessionID string,
	origin string,
	rawURL string,
	path string,
) error {
	if strings.TrimSpace(eventID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(siteCode) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(environment) == "" {
		return ErrInvalidRawEvent
	}
	if tokenVersion <= 0 {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(jwtID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(eventName) == "" {
		return ErrInvalidRawEvent
	}
	if eventVersion <= 0 {
		return ErrInvalidRawEvent
	}
	if eventTimeIsZero {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(anonymousID) == "" && strings.TrimSpace(sessionID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(origin) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(rawURL) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(path) == "" {
		return ErrInvalidRawEvent
	}
	return nil
}

// NormalizeMap devuelve un mapa no nulo para propiedades y contexto.
//
// Recibe un mapa opcional del payload y devuelve el mismo mapa cuando existe,
// o un mapa vacio cuando la entrada es nil. No copia el contenido porque el
// worker solo necesita garantizar no nulidad antes de persistir.
func NormalizeMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}
