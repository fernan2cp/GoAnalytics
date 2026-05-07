package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/site"
)

// Errores de aplicacion asociados a validacion de site.
var (
	ErrValidateSiteDependencyMissing = errors.New("dependencia de validacion de site faltante")
	ErrValidateSiteInputInvalid      = errors.New("entrada de validacion de site invalida")
	ErrSiteNotAvailable              = errors.New("metadata de site no disponible")
)

// ValidateSiteUseCase obtiene y valida metadata real de site para un evento.
//
// Usa SiteCache como fuente primaria y RehydrateSiteUseCase cuando la metadata
// no esta disponible. Devuelve site.SiteConfig si el site esta activo, tracking
// habilitado, token_version coincide y origin pertenece a allowed_domains.
//
// Debe construirse con NewValidateSiteUseCase. No importa Redis, HTTP,
// PostgreSQL ni variables de entorno.
type ValidateSiteUseCase struct {
	siteCache     outbound.SiteCache
	rehydrateSite *RehydrateSiteUseCase
}

// NewValidateSiteUseCase crea el caso de uso de validacion de site.
//
// Recibe SiteCache y RehydrateSiteUseCase para obtener metadata desde cache o
// resolver interno. Devuelve una instancia lista para validar eventos crudos.
//
// No devuelve error porque solo asigna dependencias; los problemas se informan
// al ejecutar Validate.
func NewValidateSiteUseCase(siteCache outbound.SiteCache, rehydrateSite *RehydrateSiteUseCase) *ValidateSiteUseCase {
	return &ValidateSiteUseCase{
		siteCache:     siteCache,
		rehydrateSite: rehydrateSite,
	}
}

// Validate valida metadata de site contra un evento crudo.
//
// Recibe contexto y dto.RawEvent. Devuelve site.SiteConfig valida o error. Si
// la cache no contiene metadata, intenta rehidratarla antes de aplicar reglas
// de dominio.
//
// Devuelve ErrValidateSiteInputInvalid cuando faltan campos basicos y
// ErrSiteNotAvailable cuando no puede obtener metadata suficiente para decidir.
func (uc *ValidateSiteUseCase) Validate(ctx context.Context, raw dto.RawEvent) (site.SiteConfig, error) {
	if uc == nil || uc.siteCache == nil || uc.rehydrateSite == nil {
		return site.SiteConfig{}, ErrValidateSiteDependencyMissing
	}
	if strings.TrimSpace(raw.SiteCode) == "" || strings.TrimSpace(raw.Origin) == "" || strings.TrimSpace(raw.Environment) == "" {
		return site.SiteConfig{}, ErrValidateSiteInputInvalid
	}

	cacheKey := outbound.SiteCacheKey{
		SiteCode:     raw.SiteCode,
		Origin:       raw.Origin,
		Environment:  raw.Environment,
		TokenVersion: raw.TokenVersion,
	}
	config, found, err := uc.siteCache.Get(ctx, cacheKey)
	if err != nil {
		return site.SiteConfig{}, fmt.Errorf("%w: %v", ErrSiteNotAvailable, err)
	}
	if !found {
		config, err = uc.rehydrateSite.Rehydrate(ctx, cacheKey)
		if err != nil {
			return site.SiteConfig{}, fmt.Errorf("%w: %v", ErrSiteNotAvailable, err)
		}
	}
	if err := site.ValidateForEvent(config, raw.SiteCode, raw.Environment, raw.TokenVersion, raw.Origin); err != nil {
		return site.SiteConfig{}, err
	}
	return config, nil
}
