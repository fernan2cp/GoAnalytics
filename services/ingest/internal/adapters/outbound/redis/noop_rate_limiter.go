package redis

import (
	"context"
	"time"
)

// NoopRateLimiter permite toda ingesta sin tocar infraestructura externa.
//
// Se usa cuando RATE_LIMIT_ENABLED=false. Implementa el puerto RateLimiter y
// conserva el mismo contrato del caso de uso sin crear condiciones especiales
// en la aplicacion.
type NoopRateLimiter struct{}

// Allow siempre permite la accion solicitada.
//
// Recibe los mismos parametros que el limitador real y devuelve true sin error.
// Debe usarse solo por configuracion explicita.
func (NoopRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	_ = ctx
	_ = key
	_ = limit
	_ = window
	return true, nil
}
