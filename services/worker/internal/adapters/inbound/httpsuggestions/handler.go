package httpsuggestions

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/application/usecases"
)

// Handler valida y sirve el contrato interno de sugerencias.
type Handler struct {
	enabled                bool
	token                  string
	defaultLimit, maxLimit int
	query                  *usecases.QuerySuggestions
	metrics                outbound.Metrics
}

// NewHandler crea el adaptador HTTP interno.
// Recibe habilitación, Bearer, límites, consulta y métricas opcionales. Devuelve
// el handler sin I/O; la configuración debe validarse antes en bootstrap.
func NewHandler(enabled bool, token string, defaultLimit, maxLimit int, query *usecases.QuerySuggestions, metrics ...outbound.Metrics) *Handler {
	handler := &Handler{enabled: enabled, token: token, defaultLimit: defaultLimit, maxLimit: maxLimit, query: query}
	if len(metrics) > 0 {
		handler.metrics = metrics[0]
	}
	return handler
}

// ServeHTTP autentica, valida query estricta y serializa resultados.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	started := time.Now()
	observed := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
	writer = observed
	source := "none"
	defer func() {
		if handler.metrics != nil {
			status := strconv.Itoa(observed.status)
			handler.metrics.IncrementCounter("subject_query_requests_total", map[string]string{"status": status, "source_level": source})
			handler.metrics.ObserveHistogram("subject_query_duration_seconds", time.Since(started).Seconds(), map[string]string{"status": status})
		}
	}()
	if !handler.enabled {
		writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	authorization := request.Header.Get("Authorization")
	provided := strings.TrimPrefix(authorization, "Bearer ")
	if !strings.HasPrefix(authorization, "Bearer ") || len(provided) != len(handler.token) || subtle.ConstantTimeCompare([]byte(provided), []byte(handler.token)) != 1 {
		writeError(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	query, code := handler.parseQuery(request)
	if code != "" {
		writeError(writer, http.StatusBadRequest, code)
		return
	}
	response, err := handler.query.Execute(request.Context(), query, time.Now().UTC())
	if errors.Is(err, usecases.ErrProjectionUnavailable) {
		writeError(writer, http.StatusServiceUnavailable, "projection_unavailable")
		return
	}
	if errors.Is(err, usecases.ErrProjectionStale) {
		writeError(writer, http.StatusServiceUnavailable, "projection_stale")
		return
	}
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "query_unavailable")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	source = "global"
	if level, ok := response.Context["source_level"].(string); ok {
		source = level
	}
	if handler.metrics != nil && response.IsStale {
		handler.metrics.IncrementCounter("subject_query_stale_total", nil)
	}
	_ = json.NewEncoder(writer).Encode(response)
}

// statusWriter conserva el estado HTTP para métricas sin incluir datos del cliente.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader delega el estado y lo conserva para observar la petición.
func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (handler *Handler) parseQuery(request *http.Request) (usecases.SuggestionQuery, string) {
	allowed := map[string]bool{"tenant_id": true, "subject_type": true, "context_key": true, "context_version": true, "limit": true}
	for key, values := range request.URL.Query() {
		if !allowed[key] || len(values) != 1 {
			return usecases.SuggestionQuery{}, "invalid_query"
		}
	}
	tenant := strings.TrimSpace(request.URL.Query().Get("tenant_id"))
	if tenant == "" || len(tenant) > 255 || request.URL.Query().Get("subject_type") != "customer" {
		return usecases.SuggestionQuery{}, "invalid_query"
	}
	contextKey := strings.TrimSpace(request.URL.Query().Get("context_key"))
	versionText := request.URL.Query().Get("context_version")
	contextVersion := 0
	if (contextKey == "") != (versionText == "") {
		return usecases.SuggestionQuery{}, "invalid_query"
	}
	if len(contextKey) > 255 {
		return usecases.SuggestionQuery{}, "invalid_query"
	}
	if versionText != "" {
		parsed, err := strconv.Atoi(versionText)
		if err != nil || parsed < 1 {
			return usecases.SuggestionQuery{}, "invalid_query"
		}
		contextVersion = parsed
	}
	limit := handler.defaultLimit
	if text := request.URL.Query().Get("limit"); text != "" {
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return usecases.SuggestionQuery{}, "invalid_query"
		}
		limit = parsed
	}
	if limit < 1 || limit > handler.maxLimit {
		return usecases.SuggestionQuery{}, "invalid_query"
	}
	return usecases.SuggestionQuery{TenantID: tenant, Limit: limit, ContextKey: contextKey, ContextVersion: contextVersion}, ""
}

func writeError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}
