package site

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// Errores de dominio para validar metadata de site.
//
// Permiten que la capa de aplicacion registre rechazos con motivos estables sin
// importar detalles de cache, resolver interno ni almacenamiento.
var (
	ErrInvalidConfig      = errors.New("metadata de site invalida")
	ErrSiteInactive       = errors.New("site inactivo")
	ErrTrackingDisabled   = errors.New("tracking deshabilitado")
	ErrTokenVersion       = errors.New("version de token invalida")
	ErrDomainNotAllowed   = errors.New("dominio no permitido")
	ErrEnvironmentEmpty   = errors.New("entorno vacio")
	ErrInvalidSource      = errors.New("origen de evento invalido")
	ErrTimezoneUnresolved = errors.New("zona horaria IANA no resuelta")
)

const activeStatus = "active"

const (
	// SourceWeb identifica resoluciones originadas por el endpoint publico.
	SourceWeb = "web"
	// SourceServer identifica resoluciones server-to-server sin navegacion.
	SourceServer = "server"
)

// ValidateConfig valida que la metadata de site tenga datos minimos usables.
//
// Recibe SiteConfig obtenido desde cache o resolver y comprueba site_code,
// tenant_id, site_id, estado, dominios permitidos, token_version,
// sample_rate y schema_version. Devuelve ErrInvalidConfig si falta algun dato
// requerido o si los valores numericos no son aceptables.
//
// Debe ejecutarse antes de usar la metadata para construir eventos validados.
// No consulta infraestructura ni normaliza persistencia.
func ValidateConfig(config SiteConfig) error {
	if strings.TrimSpace(config.SiteCode) == "" {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(config.TenantID) == "" {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(config.SiteID) == "" {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(config.Status) == "" {
		return ErrInvalidConfig
	}
	if len(config.AllowedDomains) == 0 {
		return ErrInvalidConfig
	}
	if config.TokenVersion <= 0 {
		return ErrInvalidConfig
	}
	if config.SampleRate < 0 || config.SampleRate > 1 {
		return ErrInvalidConfig
	}
	if config.SchemaVersion <= 0 {
		return ErrInvalidConfig
	}
	return nil
}

// ValidateForEvent valida reglas de site contra un evento crudo.
//
// Recibe SiteConfig, site_code del evento, entorno, version de token y origin.
// Devuelve errores de dominio cuando el site no coincide, esta inactivo, tiene
// tracking apagado, la version del token difiere o el dominio no esta
// permitido.
//
// Debe usarse desde application despues de obtener metadata valida. No conoce
// Redis, HTTP ni base de datos.
func ValidateForEvent(config SiteConfig, eventSiteCode string, environment string, tokenVersion int, origin string) error {
	return ValidateForEventSource(config, eventSiteCode, environment, tokenVersion, origin, SourceWeb)
}

// ValidateForEventSource valida metadata segun el origen del evento.
//
// Source vacio equivale a web. Para web conserva validacion de token y dominio;
// para server permite origin vacio y no exige token de tracking. Ambos origenes
// validan identidad, estado, tracking y entorno. Devuelve ErrInvalidSource para
// cualquier valor fuera del contrato.
func ValidateForEventSource(config SiteConfig, eventSiteCode string, environment string, tokenVersion int, origin string, source string) error {
	if err := ValidateConfig(config); err != nil {
		return err
	}
	if strings.TrimSpace(environment) == "" {
		return ErrEnvironmentEmpty
	}
	if strings.TrimSpace(config.SiteCode) != strings.TrimSpace(eventSiteCode) {
		return ErrInvalidConfig
	}
	if !strings.EqualFold(strings.TrimSpace(config.Status), activeStatus) {
		return ErrSiteInactive
	}
	if !config.TrackingEnabled {
		return ErrTrackingDisabled
	}
	source, valid := NormalizeSource(source)
	if !valid {
		return ErrInvalidSource
	}
	if source == SourceWeb {
		if config.TokenVersion != tokenVersion {
			return ErrTokenVersion
		}
		if !AllowsOrigin(config, origin) {
			return ErrDomainNotAllowed
		}
	}
	return nil
}

// NormalizeSource normaliza el origen de resolucion del site.
//
// La cadena vacia se interpreta como web para conservar llamadas legacy.
// Devuelve el valor canonico y false si no pertenece al contrato web/server.
func NormalizeSource(source string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(source))
	if normalized == "" {
		return SourceWeb, true
	}
	if normalized != SourceWeb && normalized != SourceServer {
		return "", false
	}
	return normalized, true
}

// ValidateTimezone verifica que SiteConfig contenga una zona horaria IANA.
//
// La ausencia y los identificadores que time.LoadLocation no reconoce devuelven
// ErrTimezoneUnresolved. Esta validacion es opt-in: ValidateConfig sigue
// aceptando metadata legacy sin timezone para no afectar el procesamiento web.
func ValidateTimezone(config SiteConfig) error {
	value := strings.TrimSpace(config.Timezone)
	if value == "" {
		return ErrTimezoneUnresolved
	}
	if _, err := time.LoadLocation(value); err != nil {
		return ErrTimezoneUnresolved
	}
	return nil
}

// AllowsOrigin indica si el origin pertenece a los dominios permitidos.
//
// Recibe SiteConfig y un origin HTTP/HTTPS. Devuelve true cuando el host y
// puerto del origin coinciden con algun allowed_domain normalizado. Devuelve
// false ante origin vacio, URL invalida o dominios sin coincidencia.
//
// La comparacion ignora esquema, mayusculas, un punto final del host y prefijo
// www. Conserva el puerto si existe. No acepta subdominios implicitos: cada
// dominio debe estar listado explicitamente.
func AllowsOrigin(config SiteConfig, origin string) bool {
	host := normalizedHost(origin)
	if host == "" {
		return false
	}
	for _, allowed := range config.AllowedDomains {
		if host == normalizeDomain(allowed) {
			return true
		}
	}
	return false
}

// normalizedHost extrae y normaliza el host desde un origin.
//
// Recibe una URL absoluta o un host directo y devuelve host con puerto opcional,
// en minusculas, sin punto final y sin prefijo www. Devuelve cadena vacia
// cuando no puede parsearse.
func normalizedHost(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	if parsed.Host != "" {
		return normalizeDomain(parsed.Host)
	}
	if parsed.Scheme == "" && !strings.Contains(origin, "/") {
		return normalizeDomain(origin)
	}
	return ""
}

// normalizeDomain normaliza un dominio para comparaciones exactas.
//
// Recibe un dominio posiblemente con esquema, espacios, mayusculas, punto final
// o prefijo www. Devuelve host canonico con puerto opcional. No valida DNS.
func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if parsed, err := url.Parse(domain); err == nil && parsed.Host != "" {
		domain = parsed.Host
	}
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return strings.TrimPrefix(domain, "www.")
}
