package event

import (
	"errors"
	"strings"
)

// ErrInvalidRawEvent indica que un evento crudo no cumple las reglas minimas.
//
// Se usa como error sentinela para que la capa de aplicacion pueda distinguir
// rechazos de payload frente a fallas de infraestructura. No expone detalles de
// transporte ni de adaptadores.
var ErrInvalidRawEvent = errors.New("evento crudo invalido")

// ValidateRawEvent valida las reglas minimas de un evento aceptable.
//
// Recibe un RawEvent ya construido por la capa de aplicacion y comprueba que
// tenga identificador, nombre, version positiva, tiempo de evento, identidad
// anonima o sesion, origen, URL y path. Devuelve ErrInvalidRawEvent cuando
// falta algun dato requerido.
//
// Debe ejecutarse antes de publicar el evento hacia el puerto outbound. No
// valida reglas de site ni persistencia; esas pertenecen al worker.
func ValidateRawEvent(raw RawEvent) error {
	if strings.TrimSpace(raw.EventID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.SitePublicID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.Environment) == "" {
		return ErrInvalidRawEvent
	}
	if raw.TokenVersion <= 0 {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.JWTID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.EventName) == "" {
		return ErrInvalidRawEvent
	}
	if raw.EventVersion <= 0 {
		return ErrInvalidRawEvent
	}
	if raw.EventTime.IsZero() {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.AnonymousID) == "" && strings.TrimSpace(raw.SessionID) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.Origin) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.URL) == "" {
		return ErrInvalidRawEvent
	}
	if strings.TrimSpace(raw.Path) == "" {
		return ErrInvalidRawEvent
	}
	return nil
}

// NormalizeMap devuelve un mapa no nulo para propiedades y contexto.
//
// Recibe un mapa opcional del payload y devuelve el mismo mapa cuando existe,
// o un mapa vacio cuando la entrada es nil. No copia el contenido porque Fase 1
// solo garantiza no nulidad; limites de profundidad y claves quedan para una
// regla posterior.
func NormalizeMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}
