package httpresolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	redisadapter "goanalytics/services/worker/internal/adapters/outbound/redis"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/site"
)

// Errores estables del resolver HTTP de sites.
var (
	// ErrResolverURLRequired indica que no se configuro la URL interna.
	ErrResolverURLRequired = errors.New("url de resolver requerida")

	// ErrSiteNotFound indica que el backend interno no encontro el site.
	ErrSiteNotFound = errors.New("site no encontrado")

	// ErrResolverRejected indica que el backend interno rechazo la solicitud.
	ErrResolverRejected = errors.New("resolver rechazo la solicitud")
)

// SiteResolver implementa rehidratacion de site mediante HTTP interno.
//
// Contiene URL, token, cliente HTTP y adaptadores opcionales para cooldown y
// negative cache. Implementa el puerto SiteResolver sin acoplar el worker a un
// framework especifico del backend principal.
type SiteResolver struct {
	url       string
	token     string
	client    *http.Client
	gate      *redisadapter.RehydrationGate
	siteCache *redisadapter.SiteCache
}

// NewSiteResolver crea un resolver HTTP de metadata de site.
//
// Recibe URL interna, token bearer, timeout, gate de cooldown y cache Redis.
// Devuelve error si la URL esta vacia. Si timeout no es positivo usa el valor
// por defecto de http.Client.
func NewSiteResolver(
	url string,
	token string,
	timeout time.Duration,
	gate *redisadapter.RehydrationGate,
	siteCache *redisadapter.SiteCache,
) (*SiteResolver, error) {
	if strings.TrimSpace(url) == "" {
		return nil, ErrResolverURLRequired
	}
	return &SiteResolver{
		url:       strings.TrimSpace(url),
		token:     token,
		client:    &http.Client{Timeout: timeout},
		gate:      gate,
		siteCache: siteCache,
	}, nil
}

// Resolve solicita metadata real al backend interno.
//
// Recibe contexto y ResolveSiteInput. Devuelve SiteConfig cuando el backend
// responde exitosamente. Aplica cooldown antes de llamar y marca negative cache
// cuando el backend informa que el site no existe.
func (resolver *SiteResolver) Resolve(ctx context.Context, input outbound.ResolveSiteInput) (site.SiteConfig, error) {
	if resolver == nil || resolver.client == nil {
		return site.SiteConfig{}, ErrResolverURLRequired
	}
	if resolver.gate != nil {
		if err := resolver.gate.Allow(ctx, input.SiteCode); err != nil {
			return site.SiteConfig{}, err
		}
	}

	body, err := json.Marshal(resolveRequest{
		SiteCode:     input.SiteCode,
		Origin:       input.Origin,
		Environment:  input.Environment,
		TokenVersion: input.TokenVersion,
	})
	if err != nil {
		return site.SiteConfig{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolver.url, bytes.NewReader(body))
	if err != nil {
		return site.SiteConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(resolver.token) != "" {
		req.Header.Set("Authorization", "Bearer "+resolver.token)
	}

	resp, err := resolver.client.Do(req)
	if err != nil {
		return site.SiteConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		_ = resolver.markNotFound(ctx, input.SiteCode)
		return site.SiteConfig{}, ErrSiteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return site.SiteConfig{}, fmt.Errorf("%w: status %d", ErrResolverRejected, resp.StatusCode)
	}

	var payload resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return site.SiteConfig{}, err
	}
	if payload.Error != "" {
		if payload.Error == "site_not_found" {
			_ = resolver.markNotFound(ctx, input.SiteCode)
			return site.SiteConfig{}, ErrSiteNotFound
		}
		return site.SiteConfig{}, fmt.Errorf("%w: %s", ErrResolverRejected, payload.Error)
	}
	return payload.toDomain(input), nil
}

// markNotFound guarda negative cache cuando el resolver confirma ausencia.
//
// Recibe contexto y site_code. Devuelve error si el adaptador de cache falla.
// Si no hay cache configurada, no realiza ninguna accion.
func (resolver *SiteResolver) markNotFound(ctx context.Context, siteCode string) error {
	if resolver == nil || resolver.siteCache == nil {
		return nil
	}
	return resolver.siteCache.MarkNotFound(ctx, siteCode)
}

type resolveRequest struct {
	SiteCode     string `json:"site_code"`
	Origin       string `json:"origin"`
	Environment  string `json:"env"`
	TokenVersion int    `json:"token_version"`
}

type resolveResponse struct {
	SiteCode        string   `json:"site_code"`
	TenantID        string   `json:"tenant_id"`
	SiteID          string   `json:"site_id"`
	Status          string   `json:"status"`
	TrackingEnabled bool     `json:"tracking_enabled"`
	AllowedDomains  []string `json:"allowed_domains"`
	TokenVersion    int      `json:"token_version"`
	SampleRate      float64  `json:"sample_rate"`
	SchemaVersion   int      `json:"schema_version"`
	Error           string   `json:"error"`
}

func (payload resolveResponse) toDomain(input outbound.ResolveSiteInput) site.SiteConfig {
	tokenVersion := payload.TokenVersion
	if tokenVersion <= 0 {
		tokenVersion = input.TokenVersion
	}
	sampleRate := payload.SampleRate
	if sampleRate <= 0 {
		sampleRate = 1
	}
	schemaVersion := payload.SchemaVersion
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	return site.SiteConfig{
		SiteCode:        payload.SiteCode,
		TenantID:        payload.TenantID,
		SiteID:          payload.SiteID,
		Status:          payload.Status,
		TrackingEnabled: payload.TrackingEnabled,
		AllowedDomains:  payload.AllowedDomains,
		TokenVersion:    tokenVersion,
		SampleRate:      sampleRate,
		SchemaVersion:   schemaVersion,
	}
}
