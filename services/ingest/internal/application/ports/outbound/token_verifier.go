package outbound

import (
	"context"

	"goanalytics/services/ingest/internal/domain/token"
)

// EventTokenVerifier define el puerto para validar tokens de tracking.
//
// Recibe un contexto y el token en texto recibido por el adaptador inbound.
// Devuelve token.TrackingClaims cuando la firma, algoritmo y claims son
// validos; devuelve error cuando el token falta, expiro o no cumple contrato.
//
// Debe implementarse en adaptadores outbound, inicialmente con JWT HS256 y
// luego con algoritmos asimetricos sin cambiar el caso de uso.
type EventTokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (token.TrackingClaims, error)
}
