package dto

import "time"

// IngestRequest representa la entrada del caso de uso de ingesta.
//
// Incluye el token recibido, datos derivados del request HTTP y la lista de
// eventos enviados por el SDK. Es un DTO de aplicacion, por lo que no debe
// depender de tipos HTTP ni de adaptadores concretos.
//
// Debe construirse en el adaptador inbound. No valida por si mismo ni devuelve
// errores; las validaciones ocurren en el caso de uso y en el dominio.
type IngestRequest struct {
	Token     string
	IPHash    string
	UserAgent string
	Events    []IngestEvent
}

// IngestEvent representa un evento individual recibido desde el SDK.
//
// Contiene campos de identificacion, tiempo, navegacion, propiedades y
// contexto. Se usa como tipo intermedio antes de enriquecer el evento con
// claims del JWT.
//
// Debe incluir al menos identificador, nombre, timestamp y datos necesarios
// para validar origen en fases posteriores. No devuelve errores por si mismo.
type IngestEvent struct {
	EventID                string
	LogicalEventID         string
	IdempotencyKey         string
	TabID                  string
	Sequence               int64
	PreviousLogicalEventID string
	EventName              string
	EventVersion           int
	Timestamp              time.Time
	AnonymousID            string
	SessionID              string
	UserID                 *string
	Origin                 string
	URL                    string
	Path                   string
	Referrer               string
	Properties             map[string]any
	Context                map[string]any
}
