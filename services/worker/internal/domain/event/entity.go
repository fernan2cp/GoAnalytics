package event

import "time"

// ValidatedEvent representa un evento validado y listo para persistencia.
//
// Combina el evento crudo con metadata real del site, tenant, entorno y datos
// de auditoria. Se usa como tipo de dominio para guardar eventos validos en el
// puerto EventRepository.
//
// Debe crearse solo despues de validar site activo, tracking habilitado,
// dominio permitido, version de token y deduplicacion. No devuelve errores por
// si mismo; las fallas se modelan en validaciones del worker.
type ValidatedEvent struct {
	EventID      string
	TenantID     string
	SiteID       string
	SiteCode     string
	Environment  string
	EventName    string
	EventVersion int
	EventTime    time.Time
	ReceivedAt   time.Time
	AnonymousID  string
	UserID       *string
	SessionID    string
	Origin       string
	URL          string
	Path         string
	Referrer     string
	UserAgent    string
	IPHash       string
	SDKName      string
	SDKVersion   string
	JWTID        string
	TokenVersion int
	Properties   map[string]any
	Context      map[string]any
}
