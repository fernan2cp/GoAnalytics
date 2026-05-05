package token

import "time"

// TrackingClaims representa los claims validados del JWT de tracking.
//
// Contiene identidad publica del site, entorno, version de token, tiempos de
// validez y datos opcionales de ayuda. Se usa como tipo de datos entre el
// puerto EventTokenVerifier y el caso de uso de ingesta.
//
// Debe recibirse solo despues de validar firma, algoritmo, issuer, audience,
// nbf y exp. No devuelve errores por si mismo; las fallas de token deben
// provenir del verificador concreto.
type TrackingClaims struct {
	Issuer       string
	Audience     string
	SitePublicID string
	Environment  string
	TokenVersion int
	IssuedAt     time.Time
	NotBefore    time.Time
	ExpiresAt    time.Time
	JWTID        string
	TenantHint   string
	SiteHint     string
}
