package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"goanalytics/services/ingest/internal/application/ports/inbound"
	"goanalytics/services/ingest/internal/application/ports/outbound"
	"goanalytics/services/ingest/internal/application/usecases"
)

// InternalIngestHandler atiende eventos autenticados server-to-server.
type InternalIngestHandler struct {
	ingester        inbound.IngestInternalEvents
	logger          outbound.Logger
	maxPayloadBytes int64
	tokenDigest     [sha256.Size]byte
}

// NewInternalIngestHandler crea el handler del endpoint interno.
//
// Recibe caso de uso, logger, límite y Bearer dedicado. El token se conserva
// únicamente como digest para compararlo en tiempo constante y no registrarlo.
func NewInternalIngestHandler(ingester inbound.IngestInternalEvents, logger outbound.Logger, maxPayloadBytes int64, token string) *InternalIngestHandler {
	return &InternalIngestHandler{
		ingester:        ingester,
		logger:          logger,
		maxPayloadBytes: maxPayloadBytes,
		tokenDigest:     sha256.Sum256([]byte(token)),
	}
}

// ServeHTTP procesa POST /v1/internal/events con contrato estricto.
func (handler *InternalIngestHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeInternalError(writer, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if !handler.validToken(request.Header.Get("Authorization")) {
		writeInternalError(writer, http.StatusUnauthorized, "invalid_token")
		return
	}
	if handler.maxPayloadBytes > 0 {
		request.Body = http.MaxBytesReader(writer, request.Body, handler.maxPayloadBytes)
	}
	input, err := decodeInternalIngestRequest(request)
	if err != nil {
		handler.warn(request.Context(), "payload interno inválido", err)
		writeInternalError(writer, http.StatusBadRequest, stablePayloadReason(err))
		return
	}
	response, err := handler.ingester.IngestInternal(request.Context(), input)
	if err != nil {
		handler.writeUseCaseError(writer, request.Context(), err)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(response)
}

// validToken extrae el Bearer y compara su digest en tiempo constante.
func (handler *InternalIngestHandler) validToken(header string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	received := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if received == "" {
		return false
	}
	digest := sha256.Sum256([]byte(received))
	return subtle.ConstantTimeCompare(digest[:], handler.tokenDigest[:]) == 1
}

// writeUseCaseError traduce errores internos a códigos estables sin detalles.
func (handler *InternalIngestHandler) writeUseCaseError(writer http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, usecases.ErrInvalidBatch), errors.Is(err, usecases.ErrInvalidPayload):
		handler.warn(ctx, "evento interno rechazado", err)
		writeInternalError(writer, http.StatusBadRequest, "invalid_payload")
	case errors.Is(err, usecases.ErrPublishFailed), errors.Is(err, usecases.ErrDependencyMissing):
		handler.logError(ctx, "fallo al publicar evento interno", err)
		writeInternalError(writer, http.StatusServiceUnavailable, "service_unavailable")
	default:
		handler.logError(ctx, "fallo inesperado de ingesta interna", err)
		writeInternalError(writer, http.StatusInternalServerError, "internal_error")
	}
}

// warn registra un error contractual sin incluir payload ni credenciales.
func (handler *InternalIngestHandler) warn(ctx context.Context, message string, err error) {
	if handler.logger != nil {
		handler.logger.Warn(ctx, message, withRequestID(ctx, map[string]any{"reason": err.Error()}))
	}
}

// logError registra una falla operativa sin incluir payload ni credenciales.
func (handler *InternalIngestHandler) logError(ctx context.Context, message string, err error) {
	if handler.logger != nil {
		handler.logger.Error(ctx, message, withRequestID(ctx, map[string]any{"error": err.Error()}))
	}
}

// writeInternalError escribe el formato estable de error del contrato interno.
func writeInternalError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}
