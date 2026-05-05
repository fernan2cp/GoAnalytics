package rejection

import "time"

// RejectedEvent representa un evento rechazado o sospechoso.
//
// Contiene identificadores disponibles, motivo, severidad, contexto tecnico y
// payload crudo para auditoria. Se usa como tipo de dominio para registrar
// rechazos mediante RejectedEventRepository.
//
// Debe evitar datos sensibles y nunca almacenar IP cruda. No devuelve errores
// por si mismo; las fallas corresponden a la persistencia o al armado del
// rechazo.
type RejectedEvent struct {
	EventID      string
	SitePublicID string
	Environment  string
	Reason       string
	Severity     string
	Origin       string
	URL          string
	IPHash       string
	UserAgent    string
	RawPayload   map[string]any
	CreatedAt    time.Time
}
