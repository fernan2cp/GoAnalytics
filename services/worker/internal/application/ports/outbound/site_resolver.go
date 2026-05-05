package outbound

import (
	"context"

	"goanalytics/services/worker/internal/domain/site"
)

// ResolveSiteInput representa la entrada del resolver interno de site.
//
// Incluye site_public_id, origin y environment para solicitar rehidratacion de
// metadata. Se usa como DTO de aplicacion hacia el puerto SiteResolver.
//
// Debe contener datos suficientes para que el backend principal resuelva el
// site real. No devuelve errores por si mismo.
type ResolveSiteInput struct {
	SitePublicID string
	Origin       string
	Environment  string
}

// SiteResolver define el puerto para rehidratar metadata de site.
//
// Recibe contexto y ResolveSiteInput. Devuelve site.SiteConfig cuando el
// backend interno encuentra metadata valida, o error cuando el site no existe,
// el backend rechaza la solicitud o hay problemas de comunicacion.
//
// Debe implementarse como llamada HTTP configurable u otro mecanismo interno
// sin acoplar el worker a FastAPI.
type SiteResolver interface {
	Resolve(ctx context.Context, input ResolveSiteInput) (site.SiteConfig, error)
}
