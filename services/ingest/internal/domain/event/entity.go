package event

import "time"

// RawEvent representa un evento aceptado por la ingesta antes de persistirse.
//
// Contiene datos enviados por el SDK y datos enriquecidos por el servicio,
// como hora de recepcion, user agent e IP hasheada. Se usa como tipo de datos
// de dominio para publicar eventos crudos hacia el puerto EventPublisher.
//
// Debe construirse despues de validar la estructura minima del payload. No
// contiene comportamiento ni devuelve errores; las condiciones invalidas se
// representan mediante validaciones externas del dominio o caso de uso.
type RawEvent struct {
	EventID      string
	SiteCode     string
	Environment  string
	TokenVersion int
	JWTID        string
	EventName    string
	EventVersion int
	EventTime    time.Time
	ReceivedAt   time.Time
	AnonymousID  string
	SessionID    string
	UserID       *string
	Origin       string
	URL          string
	Path         string
	Referrer     string
	UserAgent    string
	IPHash       string
	SDKName      string
	SDKVersion   string
	Properties   map[string]any
	Context      map[string]any
}
