package token

import (
	"errors"
	"strings"
	"time"
)

// ErrInvalidTrackingClaims indica que los claims de tracking no son usables.
//
// Se usa después de la verificacion criptografica del token para separar las
// reglas de dominio de la validacion concreta de JWT. No contiene detalles de
// librerias ni algoritmos.
var ErrInvalidTrackingClaims = errors.New("claims de tracking invalidos")

// Validate comprueba la consistencia minima de los claims de tracking.
//
// Recibe el tiempo actual para evaluar ventana de validez y revisa identidad
// del site, entorno, version, issuer, audience, jti, nbf y exp. Devuelve
// ErrInvalidTrackingClaims cuando falta un claim obligatorio o cuando el token
// aún no es válido o ya expiro.
//
// Debe llamarse desde la capa de aplicación después de EventTokenVerifier. La
// firma, algoritmo y parseo del JWT pertenecen al adaptador outbound.
func (claims TrackingClaims) Validate(now time.Time) error {
	if strings.TrimSpace(claims.SiteCode) == "" {
		return ErrInvalidTrackingClaims
	}
	if strings.TrimSpace(claims.Environment) == "" {
		return ErrInvalidTrackingClaims
	}
	if claims.TokenVersion <= 0 {
		return ErrInvalidTrackingClaims
	}
	if strings.TrimSpace(claims.JWTID) == "" {
		return ErrInvalidTrackingClaims
	}
	if strings.TrimSpace(claims.Issuer) == "" {
		return ErrInvalidTrackingClaims
	}
	if strings.TrimSpace(claims.Audience) == "" {
		return ErrInvalidTrackingClaims
	}
	if claims.NotBefore.IsZero() || claims.ExpiresAt.IsZero() {
		return ErrInvalidTrackingClaims
	}
	if !claims.NotBefore.Before(claims.ExpiresAt) {
		return ErrInvalidTrackingClaims
	}
	if now.Before(claims.NotBefore) {
		return ErrInvalidTrackingClaims
	}
	if !now.Before(claims.ExpiresAt) {
		return ErrInvalidTrackingClaims
	}
	return nil
}
