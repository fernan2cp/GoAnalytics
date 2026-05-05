package http

import "context"

type contextKey string

const requestIDContextKey contextKey = "request_id"

// RequestIDFromContext obtiene el identificador de request del contexto.
//
// Recibe un context.Context y devuelve el request_id asignado por middleware,
// o una cadena vacia cuando no existe. No devuelve error porque la ausencia del
// valor no impide procesar la solicitud.
func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey).(string)
	return value
}
