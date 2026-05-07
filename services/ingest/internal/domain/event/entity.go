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
	EventID                string         `json:"event_id"`
	LogicalEventID         string         `json:"logical_event_id,omitempty"`
	IdempotencyKey         string         `json:"idempotency_key,omitempty"`
	TabID                  string         `json:"tab_id,omitempty"`
	Sequence               int64          `json:"sequence,omitempty"`
	PreviousLogicalEventID string         `json:"previous_logical_event_id,omitempty"`
	SiteCode               string         `json:"site_code"`
	Environment            string         `json:"env"`
	TokenVersion           int            `json:"token_version"`
	JWTID                  string         `json:"jwt_id"`
	EventName              string         `json:"event_name"`
	EventVersion           int            `json:"event_version"`
	EventTime              time.Time      `json:"event_time"`
	ReceivedAt             time.Time      `json:"received_at"`
	AnonymousID            string         `json:"anonymous_id"`
	SessionID              string         `json:"session_id"`
	UserID                 *string        `json:"user_id"`
	Origin                 string         `json:"origin"`
	URL                    string         `json:"url"`
	Path                   string         `json:"path"`
	Referrer               string         `json:"referrer"`
	UserAgent              string         `json:"user_agent"`
	IPHash                 string         `json:"ip_hash"`
	SDKName                string         `json:"sdk_name"`
	SDKVersion             string         `json:"sdk_version"`
	Properties             map[string]any `json:"properties"`
	Context                map[string]any `json:"context"`
}
