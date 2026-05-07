package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/site"
)

// Errores de aplicacion asociados a la rehidratacion de site.
var (
	ErrRehydrateDependencyMissing = errors.New("dependencia de rehidratacion faltante")
	ErrRehydrateInputInvalid      = errors.New("entrada de rehidratacion invalida")
	ErrSiteResolveFailed          = errors.New("rehidratacion de site fallida")
)

// RehydrateSiteOptions define la politica de cache para rehidratacion.
//
// Recibe el TTL usado al guardar metadata resuelta. Si SiteCacheTTL es cero o
// negativo, se usa DefaultSiteCacheTTL. La estructura se inyecta desde
// bootstrap y evita leer variables de entorno desde application.
type RehydrateSiteOptions struct {
	SiteCacheTTL time.Duration
}

const (
	// DefaultSiteCacheTTL es el TTL inicial para metadata de site en Fase 3.
	DefaultSiteCacheTTL = time.Hour
)

// RehydrateSiteUseCase obtiene metadata real de site desde un resolver interno.
//
// Usa SiteResolver para consultar una URL interna abstracta y SiteCache para
// guardar el resultado. Devuelve site.SiteConfig validado por dominio o error
// cuando la entrada, el resolver, la cache o la metadata son invalidos.
//
// Debe construirse con NewRehydrateSiteUseCase. No importa HTTP, Redis ni
// variables de entorno.
type RehydrateSiteUseCase struct {
	siteResolver outbound.SiteResolver
	siteCache    outbound.SiteCache
	options      RehydrateSiteOptions
}

// NewRehydrateSiteUseCase crea el caso de uso de rehidratacion.
//
// Recibe puertos de resolver y cache junto con opciones de TTL. Devuelve una
// instancia lista para consultar metadata faltante y renovar la cache.
//
// No devuelve error porque solo asigna dependencias; los problemas se reportan
// al ejecutar Rehydrate.
func NewRehydrateSiteUseCase(
	siteResolver outbound.SiteResolver,
	siteCache outbound.SiteCache,
	options RehydrateSiteOptions,
) *RehydrateSiteUseCase {
	return &RehydrateSiteUseCase{
		siteResolver: siteResolver,
		siteCache:    siteCache,
		options:      normalizeRehydrateOptions(options),
	}
}

// Rehydrate resuelve y cachea metadata de site.
//
// Recibe contexto y clave de cache con site_code, origin, entorno y
// token_version. Devuelve site.SiteConfig cuando el resolver responde metadata
// valida y esta puede guardarse en cache.
//
// Devuelve ErrRehydrateInputInvalid para entradas incompletas,
// ErrSiteResolveFailed para fallas del resolver y errores de dominio o cache
// cuando la metadata no es usable o no puede persistirse temporalmente.
func (uc *RehydrateSiteUseCase) Rehydrate(ctx context.Context, key outbound.SiteCacheKey) (site.SiteConfig, error) {
	if uc == nil || uc.siteResolver == nil || uc.siteCache == nil {
		return site.SiteConfig{}, ErrRehydrateDependencyMissing
	}
	if strings.TrimSpace(key.SiteCode) == "" ||
		strings.TrimSpace(key.Origin) == "" ||
		strings.TrimSpace(key.Environment) == "" ||
		key.TokenVersion <= 0 {
		return site.SiteConfig{}, ErrRehydrateInputInvalid
	}

	config, err := uc.siteResolver.Resolve(ctx, outbound.ResolveSiteInput{
		SiteCode:     key.SiteCode,
		Origin:       key.Origin,
		Environment:  key.Environment,
		TokenVersion: key.TokenVersion,
	})
	if err != nil {
		return site.SiteConfig{}, fmt.Errorf("%w: %v", ErrSiteResolveFailed, err)
	}
	if err := site.ValidateConfig(config); err != nil {
		return site.SiteConfig{}, err
	}
	if err := uc.siteCache.Set(ctx, key, config, uc.options.SiteCacheTTL); err != nil {
		return site.SiteConfig{}, err
	}
	return config, nil
}

// normalizeRehydrateOptions completa defaults del caso de rehidratacion.
//
// Recibe opciones parciales y devuelve una copia lista para usar sin consultar
// infraestructura.
func normalizeRehydrateOptions(options RehydrateSiteOptions) RehydrateSiteOptions {
	if options.SiteCacheTTL <= 0 {
		options.SiteCacheTTL = DefaultSiteCacheTTL
	}
	return options
}
