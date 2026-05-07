package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"goanalytics/services/ingest/internal/application/ports/inbound"
	"goanalytics/services/ingest/internal/application/ports/outbound"
	"goanalytics/services/ingest/internal/application/usecases"
)

// IngestHandler atiende el endpoint publico de ingesta de eventos.
//
// Contiene el puerto inbound de aplicacion, logger y limite de bytes del
// payload. No contiene reglas de negocio; solo traduce HTTP a DTOs y errores a
// codigos de respuesta.
type IngestHandler struct {
	ingester         inbound.IngestEvents
	logger           outbound.Logger
	maxPayloadBytes  int64
	hideAuthFailures bool
}

// NewIngestHandler crea el handler HTTP de ingesta.
//
// Recibe el puerto inbound, logger, limite de payload y politica de errores de
// autenticacion. Devuelve un handler listo para registrarse en el router.
func NewIngestHandler(ingester inbound.IngestEvents, logger outbound.Logger, maxPayloadBytes int64, hideAuthFailures bool) *IngestHandler {
	return &IngestHandler{
		ingester:         ingester,
		logger:           logger,
		maxPayloadBytes:  maxPayloadBytes,
		hideAuthFailures: hideAuthFailures,
	}
}

// ServeHTTP procesa POST /v1/events.
//
// Recibe solicitudes HTTP con JSON de eventos y responde 202 con conteos cuando
// el caso de uso acepta el batch para procesamiento. Devuelve errores HTTP solo
// para fallas de contrato global o infraestructura.
func (handler *IngestHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "metodo no permitido", http.StatusMethodNotAllowed)
		return
	}
	if handler.maxPayloadBytes > 0 {
		request.Body = http.MaxBytesReader(writer, request.Body, handler.maxPayloadBytes)
	}

	input, err := decodeIngestRequest(request)
	if err != nil {
		handler.warn(request.Context(), "payload de ingesta invalido", err)
		http.Error(writer, "payload invalido", http.StatusBadRequest)
		return
	}

	response, err := handler.ingester.Ingest(request.Context(), input)
	if err != nil {
		handler.writeUseCaseError(writer, request.Context(), err)
		return
	}
	handler.info(request.Context(), "eventos evaluados para procesamiento", map[string]any{
		"accepted": response.Accepted,
		"rejected": response.Rejected,
	})
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(response)
}

// writeUseCaseError traduce errores de aplicacion a respuestas HTTP.
//
// Recibe writer, contexto y error del caso de uso. No expone detalles internos
// ni secretos en la respuesta.
func (handler *IngestHandler) writeUseCaseError(writer http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, usecases.ErrInvalidToken):
		handler.warn(ctx, "token de ingesta invalido", err)
		if handler.hideAuthFailures {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(writer, "token invalido", http.StatusUnauthorized)
	case errors.Is(err, usecases.ErrInvalidBatch), errors.Is(err, usecases.ErrInvalidPayload):
		handler.warn(ctx, "payload de ingesta rechazado", err)
		http.Error(writer, "payload invalido", http.StatusBadRequest)
	case errors.Is(err, usecases.ErrRateLimitExceeded):
		handler.warn(ctx, "limite de ingesta excedido", err)
		http.Error(writer, "limite excedido", http.StatusTooManyRequests)
	case errors.Is(err, usecases.ErrPublishFailed), errors.Is(err, usecases.ErrDependencyMissing):
		handler.error(ctx, "fallo de infraestructura de ingesta", err)
		http.Error(writer, "servicio no disponible", http.StatusServiceUnavailable)
	default:
		handler.error(ctx, "fallo inesperado de ingesta", err)
		http.Error(writer, "error interno", http.StatusInternalServerError)
	}
}

// info registra una entrada informativa asociada al request.
func (handler *IngestHandler) info(ctx context.Context, message string, attrs map[string]any) {
	if handler.logger == nil {
		return
	}
	attrs = withRequestID(ctx, attrs)
	handler.logger.Info(ctx, message, attrs)
}

// warn registra una advertencia asociada al request.
func (handler *IngestHandler) warn(ctx context.Context, message string, err error) {
	if handler.logger == nil {
		return
	}
	handler.logger.Warn(ctx, message, withRequestID(ctx, map[string]any{"error": err.Error()}))
}

// error registra una falla asociada al request.
func (handler *IngestHandler) error(ctx context.Context, message string, err error) {
	if handler.logger == nil {
		return
	}
	handler.logger.Error(ctx, message, withRequestID(ctx, map[string]any{"error": err.Error()}))
}

// withRequestID agrega el request_id a los atributos de log.
func withRequestID(ctx context.Context, attrs map[string]any) map[string]any {
	if attrs == nil {
		attrs = map[string]any{}
	}
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		attrs["request_id"] = requestID
	}
	return attrs
}
