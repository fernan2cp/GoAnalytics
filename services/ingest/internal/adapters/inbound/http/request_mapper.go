package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	EventID                string         `json:"event_id"`
	LogicalEventID         string         `json:"logical_event_id"`
	IdempotencyKey         string         `json:"idempotency_key"`
	TabID                  string         `json:"tab_id"`
	Sequence               int64          `json:"sequence"`
	PreviousLogicalEventID string         `json:"previous_logical_event_id"`
	EventName              string         `json:"event_name"`
	EventVersion           int            `json:"event_version"`
	Timestamp              time.Time      `json:"timestamp"`
	AnonymousID            string         `json:"anonymous_id"`
	SessionID              string         `json:"session_id"`
	UserID                 *string        `json:"user_id"`
	Origin                 string         `json:"origin"`
	URL                    string         `json:"url"`
	Path                   string         `json:"path"`
	Referrer               string         `json:"referrer"`
	Properties             map[string]any `json:"properties"`
	Context                map[string]any `json:"context"`
}

type publicEventPayload struct {
	EventID                string         `json:"event_id"`
	LogicalEventID         string         `json:"logical_event_id"`
	IdempotencyKey         string         `json:"idempotency_key"`
	TabID                  string         `json:"tab_id"`
	Sequence               int64          `json:"sequence"`
	PreviousLogicalEventID string         `json:"previous_logical_event_id"`
	EventName              string         `json:"event_name"`
	EventType              string         `json:"event_type"`
	EventVersion           int            `json:"event_version"`
	Timestamp              time.Time      `json:"timestamp"`
	AnonymousID            string         `json:"anonymous_id"`
	SessionID              string         `json:"session_id"`
	UserID                 *string        `json:"user_id"`
	Origin                 string         `json:"origin"`
	URL                    string         `json:"url"`
	PageURL                string         `json:"page_url"`
	Path                   string         `json:"path"`
	Referrer               string         `json:"referrer"`
	Properties             map[string]any `json:"properties"`
	Metadata               map[string]any `json:"metadata"`
	Context                map[string]any `json:"context"`
	Extra                  map[string]any `json:"-"`
}

var propertyCompatibleFields = map[string]struct{}{
	"raw":           {},
	"items":         {},
	"filters":       {},
	"route_name":    {},
	"search_term":   {},
	"previous_page": {},
}

var contextCompatibleFields = map[string]struct{}{
	"allowed_domains":  {},
	"app":              {},
	"consent":          {},
	"site_code":        {},
	"site_id":          {},
	"status":           {},
	"tenant_id":        {},
	"tracking_enabled": {},
}

var knownPublicEventFields = map[string]struct{}{
	"event_id":                  {},
	"logical_event_id":          {},
	"idempotency_key":           {},
	"tab_id":                    {},
	"sequence":                  {},
	"previous_logical_event_id": {},
	"event_name":                {},
	"event_type":                {},
	"event_version":             {},
	"timestamp":                 {},
	"anonymous_id":              {},
	"session_id":                {},
	"user_id":                   {},
	"origin":                    {},
	"url":                       {},
	"page_url":                  {},
	"path":                      {},
	"referrer":                  {},
	"properties":                {},
	"metadata":                  {},
	"context":                   {},
}

// decodeIngestRequest traduce el request HTTP al DTO de aplicacion.
//
// Recibe la solicitud HTTP y devuelve dto.IngestRequest sin ejecutar reglas de
// negocio. Devuelve error cuando el JSON no cumple la forma esperada.
func decodeIngestRequest(request *nethttp.Request) (dto.IngestRequest, error) {
	payload, err := decodePublicPayload(request)
	if err != nil {
		return dto.IngestRequest{}, err
	}

	headerOrigin := strings.TrimSpace(request.Header.Get("Origin"))
	events := make([]dto.IngestEvent, 0, len(payload.Events))
	for _, item := range payload.Events {
		origin := item.Origin
		if headerOrigin != "" {
			origin = headerOrigin
		}
		events = append(events, dto.IngestEvent{
			EventID:                item.EventID,
			LogicalEventID:         item.LogicalEventID,
			IdempotencyKey:         item.IdempotencyKey,
			TabID:                  item.TabID,
			Sequence:               item.Sequence,
			PreviousLogicalEventID: item.PreviousLogicalEventID,
			EventName:              item.EventName,
			EventVersion:           item.EventVersion,
			Timestamp:              item.Timestamp,
			AnonymousID:            item.AnonymousID,
			SessionID:              item.SessionID,
			UserID:                 item.UserID,
			Origin:                 origin,
			URL:                    item.URL,
			Path:                   item.Path,
			Referrer:               item.Referrer,
			Properties:             item.Properties,
			Context:                item.Context,
		})
	}

	return dto.IngestRequest{
		Token:     bearerToken(request.Header.Get("Authorization")),
		IPHash:    hashIP(clientIP(request)),
		UserAgent: request.UserAgent(),
		Events:    events,
	}, nil
}

// decodePublicPayload acepta el contrato batch y el contrato single-event.
//
// Recibe la solicitud HTTP y devuelve un payload publico normalizado. Mantiene
// decodificacion tolerante para integraciones que envian campos adicionales
// razonables. Devuelve error solo si el JSON o la forma global del batch son
// invalidos.
func decodePublicPayload(request *nethttp.Request) (ingestRequestPayload, error) {
	var root map[string]json.RawMessage
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&root); err != nil {
		return ingestRequestPayload{}, err
	}
	if len(root) == 0 {
		return ingestRequestPayload{}, fmt.Errorf("payload vacio")
	}

	if rawEvents, ok := root["events"]; ok {
		var publicEvents []publicEventPayload
		if err := json.Unmarshal(rawEvents, &publicEvents); err != nil {
			return ingestRequestPayload{}, err
		}
		events := make([]ingestEventPayload, 0, len(publicEvents))
		for _, item := range publicEvents {
			events = append(events, item.normalize())
		}
		return ingestRequestPayload{Events: events}, nil
	}

	var single publicEventPayload
	if err := json.Unmarshal(mustMarshalRaw(root), &single); err != nil {
		return ingestRequestPayload{}, err
	}
	single.Extra = decodeExtraFields(root)
	return ingestRequestPayload{Events: []ingestEventPayload{single.normalize()}}, nil
}

// UnmarshalJSON decodifica un evento publico y conserva campos compatibles.
//
// Recibe bytes JSON de un evento y completa la estructura publica. Devuelve
// error cuando el evento no es un objeto JSON valido.
func (payload *publicEventPayload) UnmarshalJSON(data []byte) error {
	type alias publicEventPayload
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	decoded.Extra = decodeExtraFields(raw)
	*payload = publicEventPayload(decoded)
	return nil
}

// normalize aplica alias y fusiona campos extensibles del contrato publico.
//
// No valida reglas de dominio; solo transforma nombres compatibles hacia el
// DTO interno usado por application.
func (payload publicEventPayload) normalize() ingestEventPayload {
	eventName := strings.TrimSpace(payload.EventName)
	if eventName == "" {
		eventName = payload.EventType
	}
	eventURL := strings.TrimSpace(payload.URL)
	if eventURL == "" {
		eventURL = payload.PageURL
	}
	properties := mergeMaps(payload.Properties, payload.Metadata)
	context := cloneMap(payload.Context)
	siteContext := map[string]any{}
	for key, value := range payload.Extra {
		if _, ok := propertyCompatibleFields[key]; ok {
			properties[key] = value
			continue
		}
		if _, ok := contextCompatibleFields[key]; ok {
			if isSiteMetadataField(key) {
				siteContext[key] = value
				continue
			}
			context[key] = value
		}
	}
	if len(siteContext) > 0 {
		context["site"] = mergeMaps(asMap(context["site"]), siteContext)
	}
	return ingestEventPayload{
		EventID:                payload.EventID,
		LogicalEventID:         payload.LogicalEventID,
		IdempotencyKey:         payload.IdempotencyKey,
		TabID:                  payload.TabID,
		Sequence:               payload.Sequence,
		PreviousLogicalEventID: payload.PreviousLogicalEventID,
		EventName:              eventName,
		EventVersion:           payload.EventVersion,
		Timestamp:              payload.Timestamp,
		AnonymousID:            payload.AnonymousID,
		SessionID:              payload.SessionID,
		UserID:                 payload.UserID,
		Origin:                 payload.Origin,
		URL:                    eventURL,
		Path:                   payload.Path,
		Referrer:               payload.Referrer,
		Properties:             properties,
		Context:                context,
	}
}

// decodeExtraFields extrae campos no canonicos conocidos del evento publico.
func decodeExtraFields(raw map[string]json.RawMessage) map[string]any {
	extra := map[string]any{}
	for key, value := range raw {
		if _, known := knownPublicEventFields[key]; known {
			continue
		}
		if _, ok := propertyCompatibleFields[key]; !ok {
			if _, ok := contextCompatibleFields[key]; !ok {
				continue
			}
		}
		var decoded any
		if err := json.Unmarshal(value, &decoded); err == nil {
			extra[key] = decoded
		}
	}
	return extra
}

// isSiteMetadataField indica si una clave pertenece a metadata de site.
func isSiteMetadataField(key string) bool {
	switch key {
	case "site_code", "tenant_id", "site_id", "status", "tracking_enabled", "allowed_domains":
		return true
	default:
		return false
	}
}

// asMap devuelve un mapa cuando el valor ya tiene forma de objeto JSON.
func asMap(value any) map[string]any {
	values, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return values
}

// mergeMaps fusiona propiedades con metadata, priorizando properties.
func mergeMaps(primary map[string]any, secondary map[string]any) map[string]any {
	merged := cloneMap(secondary)
	for key, value := range primary {
		merged[key] = value
	}
	return merged
}

// cloneMap copia un mapa JSON para evitar compartir referencias mutables.
func cloneMap(values map[string]any) map[string]any {
	cloned := map[string]any{}
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// mustMarshalRaw recompone un objeto JSON ya parseado para decodificarlo.
func mustMarshalRaw(raw map[string]json.RawMessage) []byte {
	data, err := json.Marshal(raw)
	if err != nil {
		return []byte("{}")
	}
	return data
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
