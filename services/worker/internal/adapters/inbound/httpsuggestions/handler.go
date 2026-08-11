package httpsuggestions

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goanalytics/services/worker/internal/application/usecases"
)

// Handler valida y sirve el contrato interno de sugerencias.
type Handler struct {
	enabled                bool
	token                  string
	defaultLimit, maxLimit int
	query                  *usecases.QuerySuggestions
}

// NewHandler crea el adaptador HTTP interno.
func NewHandler(enabled bool, token string, defaultLimit, maxLimit int, query *usecases.QuerySuggestions) *Handler {
	return &Handler{enabled: enabled, token: token, defaultLimit: defaultLimit, maxLimit: maxLimit, query: query}
}

// ServeHTTP autentica, valida query estricta y serializa resultados.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !handler.enabled {
		writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if len(provided) != len(handler.token) || subtle.ConstantTimeCompare([]byte(provided), []byte(handler.token)) != 1 {
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
	_ = json.NewEncoder(writer).Encode(response)
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
