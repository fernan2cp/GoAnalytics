package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// SlogLogger implementa logs estructurados mediante log/slog.
//
// Recibe un logger concreto de la libreria estandar y expone los metodos del
// puerto de aplicacion. Debe construirse con NewSlogLogger para aplicar nivel
// y formato JSON consistentes.
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger crea un logger estructurado para el servicio de ingesta.
//
// Recibe el nivel textual configurado por entorno y devuelve un SlogLogger que
// escribe JSON en stdout. Si el nivel no es reconocido usa info. No devuelve
// error porque la salida estandar siempre esta disponible para el proceso.
func NewSlogLogger(level string) *SlogLogger {
	options := &slog.HandlerOptions{Level: parseLevel(level)}
	return &SlogLogger{logger: slog.New(slog.NewJSONHandler(os.Stdout, options))}
}

// Info registra un evento informativo.
//
// Recibe contexto, mensaje y atributos estructurados. No devuelve error para
// no afectar el flujo de ingesta si el sink de logs falla.
func (logger *SlogLogger) Info(ctx context.Context, message string, attrs map[string]any) {
	logger.log(ctx, slog.LevelInfo, message, attrs)
}

// Warn registra una advertencia operativa.
//
// Recibe contexto, mensaje y atributos estructurados. No devuelve error y debe
// evitar atributos con secretos o datos sensibles.
func (logger *SlogLogger) Warn(ctx context.Context, message string, attrs map[string]any) {
	logger.log(ctx, slog.LevelWarn, message, attrs)
}

// Error registra una falla operativa.
//
// Recibe contexto, mensaje y atributos estructurados. No devuelve error; el
// error original debe incluirse como atributo ya sanitizado por el llamador.
func (logger *SlogLogger) Error(ctx context.Context, message string, attrs map[string]any) {
	logger.log(ctx, slog.LevelError, message, attrs)
}

// log convierte atributos de mapa a slog.Attr y delega en el logger concreto.
//
// Recibe nivel, mensaje y atributos opcionales. No retorna valores; si el
// receptor es nil crea una salida nula por seguridad.
func (logger *SlogLogger) log(ctx context.Context, level slog.Level, message string, attrs map[string]any) {
	if logger == nil || logger.logger == nil {
		return
	}
	args := make([]any, 0, len(attrs)*2)
	for key, value := range attrs {
		args = append(args, key, value)
	}
	logger.logger.Log(ctx, level, message, args...)
}

// parseLevel traduce el nivel configurado al tipo de slog.
//
// Recibe texto de entorno y devuelve slog.Level. Los valores no reconocidos
// vuelven a info para mantener un comportamiento predecible.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
