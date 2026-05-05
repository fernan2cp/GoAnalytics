package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/worker/internal/domain/site"
)

const (
	siteCachePrefix     = "goanalytics:site:public:"
	negativeCachePrefix = "goanalytics:site:not_found:"
)

// SiteCache implementa cache de metadata de site sobre Redis.
//
// Contiene un cliente Redis y el TTL de negative cache. Implementa el puerto
// SiteCache sin exponer claves, JSON ni go-redis a la capa de aplicacion.
type SiteCache struct {
	client           goredis.UniversalClient
	negativeCacheTTL time.Duration
}

// NewSiteCache crea una cache Redis para metadata de site.
//
// Recibe cliente Redis y TTL de negative cache. Devuelve error si falta el
// cliente. El TTL solo se usa cuando otros adaptadores marcan sites ausentes.
func NewSiteCache(client goredis.UniversalClient, negativeCacheTTL time.Duration) (*SiteCache, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente redis requerido")
	}
	return &SiteCache{client: client, negativeCacheTTL: negativeCacheTTL}, nil
}

// GetByPublicID obtiene metadata de site desde Redis.
//
// Recibe contexto y site_code publico. Devuelve SiteConfig, indicador de
// existencia y error ante fallas de Redis o JSON invalido. Si existe negative
// cache, devuelve found=false sin consultar el resolver.
func (cache *SiteCache) GetByPublicID(ctx context.Context, sitePublicID string) (site.SiteConfig, bool, error) {
	sitePublicID = strings.TrimSpace(sitePublicID)
	if sitePublicID == "" {
		return site.SiteConfig{}, false, nil
	}
	negative, err := cache.client.Exists(ctx, negativeCacheKey(sitePublicID)).Result()
	if err != nil {
		return site.SiteConfig{}, false, err
	}
	if negative > 0 {
		return site.SiteConfig{}, false, ErrNegativeCached
	}

	value, err := cache.client.Get(ctx, siteCacheKey(sitePublicID)).Result()
	if err == goredis.Nil {
		return site.SiteConfig{}, false, nil
	}
	if err != nil {
		return site.SiteConfig{}, false, err
	}

	var cached sitePayload
	if err := json.Unmarshal([]byte(value), &cached); err != nil {
		return site.SiteConfig{}, false, err
	}
	return cached.toDomain(), true, nil
}

// Set guarda metadata de site en Redis.
//
// Recibe contexto, SiteConfig y TTL. Serializa la metadata con el contrato
// documentado y elimina negative cache previa para el mismo site. Devuelve
// error si Redis rechaza alguna operacion.
func (cache *SiteCache) Set(ctx context.Context, config site.SiteConfig, ttl time.Duration) error {
	payload, err := json.Marshal(sitePayloadFromDomain(config))
	if err != nil {
		return err
	}
	if err := cache.client.Set(ctx, siteCacheKey(config.SiteCode), payload, ttl).Err(); err != nil {
		return err
	}
	return cache.client.Del(ctx, negativeCacheKey(config.SiteCode)).Err()
}

// MarkNotFound guarda negative cache temporal para un site inexistente.
//
// Recibe contexto y site_code publico. Devuelve error si Redis falla. Si el TTL
// no es positivo no realiza ninguna escritura.
func (cache *SiteCache) MarkNotFound(ctx context.Context, siteCode string) error {
	if cache == nil || cache.negativeCacheTTL <= 0 || strings.TrimSpace(siteCode) == "" {
		return nil
	}
	return cache.client.Set(ctx, negativeCacheKey(siteCode), "1", cache.negativeCacheTTL).Err()
}

// siteCacheKey construye la clave Redis de metadata.
//
// Recibe site_code publico y devuelve la clave estable documentada.
func siteCacheKey(siteCode string) string {
	return siteCachePrefix + strings.TrimSpace(siteCode)
}

// negativeCacheKey construye la clave Redis de negative cache.
//
// Recibe site_code publico y devuelve la clave estable documentada.
func negativeCacheKey(siteCode string) string {
	return negativeCachePrefix + strings.TrimSpace(siteCode)
}

type sitePayload struct {
	SiteCode        string   `json:"site_code"`
	TenantID        string   `json:"tenant_id"`
	SiteID          string   `json:"site_id"`
	Status          string   `json:"status"`
	TrackingEnabled bool     `json:"tracking_enabled"`
	AllowedDomains  []string `json:"allowed_domains"`
	TokenVersion    int      `json:"token_version"`
	SampleRate      float64  `json:"sample_rate"`
	SchemaVersion   int      `json:"schema_version"`
}

func (payload sitePayload) toDomain() site.SiteConfig {
	return site.SiteConfig{
		SiteCode:        payload.SiteCode,
		TenantID:        payload.TenantID,
		SiteID:          payload.SiteID,
		Status:          payload.Status,
		TrackingEnabled: payload.TrackingEnabled,
		AllowedDomains:  payload.AllowedDomains,
		TokenVersion:    payload.TokenVersion,
		SampleRate:      payload.SampleRate,
		SchemaVersion:   payload.SchemaVersion,
	}
}

func sitePayloadFromDomain(config site.SiteConfig) sitePayload {
	return sitePayload{
		SiteCode:        config.SiteCode,
		TenantID:        config.TenantID,
		SiteID:          config.SiteID,
		Status:          config.Status,
		TrackingEnabled: config.TrackingEnabled,
		AllowedDomains:  config.AllowedDomains,
		TokenVersion:    config.TokenVersion,
		SampleRate:      config.SampleRate,
		SchemaVersion:   config.SchemaVersion,
	}
}
