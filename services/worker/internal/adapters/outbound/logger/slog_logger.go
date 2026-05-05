package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// SlogLogger adapta slog al puerto de logs del worker.
//
// Recibe un logger estructurado de la libreria estandar y expone metodos
// Info, Warn y Error con atributos simples. No devuelve errores porque el
// logging no debe frenar el procesamiento del worker.
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger crea un logger estructurado para el worker.
//
// Recibe el nivel textual configurado por entorno. Devuelve una instancia
// lista para inyectar en casos de uso y adaptadores. Los niveles no
// reconocidos usan info como valor seguro.
func NewSlogLogger(level string) *SlogLogger {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
	return &SlogLogger{logger: slog.New(handler)}
}

// Info registra un mensaje informativo.
//
// Recibe contexto, mensaje y atributos estructurados. No devuelve errores; si
// el logger no esta inicializado, la llamada se ignora.
func (logger *SlogLogger) Info(ctx context.Context, message string, attrs map[string]any) {
	logger.log(ctx, slog.LevelInfo, message, attrs)
}

// Warn registra un mensaje de advertencia.
//
// Recibe contexto, mensaje y atributos estructurados. No devuelve errores; si
// el logger no esta inicializado, la llamada se ignora.
func (logger *SlogLogger) Warn(ctx context.Context, message string, attrs map[string]any) {
	logger.log(ctx, slog.LevelWarn, message, attrs)
}

// Error registra un mensaje de error.
//
// Recibe contexto, mensaje y atributos estructurados. No devuelve errores; si
// el logger no esta inicializado, la llamada se ignora.
func (logger *SlogLogger) Error(ctx context.Context, message string, attrs map[string]any) {
	logger.log(ctx, slog.LevelError, message, attrs)
}

// log normaliza atributos y delega en slog.
//
// Recibe contexto, nivel, mensaje y mapa de atributos. No devuelve errores
// porque slog no reporta fallas de escritura al llamador.
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
