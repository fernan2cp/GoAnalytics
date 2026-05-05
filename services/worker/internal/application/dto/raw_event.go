package dto

import "time"

// RawEvent representa un evento crudo consumido por el worker.
//
// Incluye campos publicados por la ingesta en el stream, ya enriquecidos con
// claims del JWT, datos de recepcion y contexto del SDK. Se usa como DTO de
// aplicacion entre el consumer y los casos de uso.
//
// Debe construirse en el adaptador de consumo. No valida por si mismo ni
// devuelve errores; las validaciones ocurren en application y domain.
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
