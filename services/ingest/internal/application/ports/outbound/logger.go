package outbound

import "context"

// Logger define el puerto minimo de logs estructurados.
//
// Cada metodo recibe contexto, mensaje y atributos tipados como map[string]any.
// No devuelve datos ni errores para evitar que el logging condicione el flujo
// principal del caso de uso.
//
// Debe implementarse en adaptadores outbound con slog, zerolog u otra libreria
// concreta. Los atributos deben evitar secretos y datos sensibles.
type Logger interface {
	Info(ctx context.Context, message string, attrs map[string]any)
	Warn(ctx context.Context, message string, attrs map[string]any)
	Error(ctx context.Context, message string, attrs map[string]any)
}
