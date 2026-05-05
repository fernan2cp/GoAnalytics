package outbound

import (
	"context"
	"time"
)

// RateLimiter define el puerto para aplicar limites de ingesta.
//
// Recibe un contexto, una clave logica, un limite numerico y una ventana de
// tiempo. Devuelve true cuando la accion esta permitida, false cuando excede
// el limite y error si la infraestructura de conteo falla.
//
// Debe implementarse en adaptadores outbound. El caso de uso decide que claves
// usar, por ejemplo site_code o IP hasheada.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}
