package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	nethttp "net/http"
	"strings"
	"time"

	"goanalytics/services/ingest/internal/application/dto"
)

// ingestRequestPayload representa el JSON publico de POST /v1/events.
type ingestRequestPayload struct {
	Events []ingestEventPayload `json:"events"`
}

// ingestEventPayload representa un evento del SDK en el contrato HTTP.
type ingestEventPayload struct {
	EventID      string         `json:"event_id"`
	EventName    string         `json:"event_name"`
	EventVersion int            `json:"event_version"`
	Timestamp    time.Time      `json:"timestamp"`
	AnonymousID  string         `json:"anonymous_id"`
	SessionID    string         `json:"session_id"`
	UserID       *string        `json:"user_id"`
	Origin       string         `json:"origin"`
	URL          string         `json:"url"`
	Path         string         `json:"path"`
	Referrer     string         `json:"referrer"`
	Properties   map[string]any `json:"properties"`
	Context      map[string]any `json:"context"`
}

// decodeIngestRequest traduce el request HTTP al DTO de aplicacion.
//
// Recibe la solicitud HTTP y devuelve dto.IngestRequest sin ejecutar reglas de
// negocio. Devuelve error cuando el JSON no cumple la forma esperada.
func decodeIngestRequest(request *nethttp.Request) (dto.IngestRequest, error) {
	var payload ingestRequestPayload
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return dto.IngestRequest{}, err
	}

	events := make([]dto.IngestEvent, 0, len(payload.Events))
	for _, item := range payload.Events {
		events = append(events, dto.IngestEvent{
			EventID:      item.EventID,
			EventName:    item.EventName,
			EventVersion: item.EventVersion,
			Timestamp:    item.Timestamp,
			AnonymousID:  item.AnonymousID,
			SessionID:    item.SessionID,
			UserID:       item.UserID,
			Origin:       item.Origin,
			URL:          item.URL,
			Path:         item.Path,
			Referrer:     item.Referrer,
			Properties:   item.Properties,
			Context:      item.Context,
		})
	}

	return dto.IngestRequest{
		Token:     bearerToken(request.Header.Get("Authorization")),
		IPHash:    hashIP(clientIP(request)),
		UserAgent: request.UserAgent(),
		Events:    events,
	}, nil
}

// bearerToken extrae el token del header Authorization.
//
// Recibe el valor completo del header y devuelve solo el token si usa el
// esquema Bearer. Para otros valores devuelve cadena vacia.
func bearerToken(header string) string {
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

// clientIP obtiene la IP del cliente sin conservar el valor crudo.
//
// Recibe la solicitud HTTP y devuelve la IP detectada desde X-Forwarded-For,
// X-Real-IP o RemoteAddr. El llamador debe hashearla antes de enviarla al caso
// de uso.
func clientIP(request *nethttp.Request) string {
	if forwarded := request.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := request.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

// hashIP aplica SHA-256 a la IP del cliente.
//
// Recibe la IP cruda detectada y devuelve un hash hexadecimal. Si la entrada
// esta vacia devuelve cadena vacia para que el caso de uso pueda rechazarla
// cuando el rate limit por IP este activo.
func hashIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}
