package outbound

import (
	"context"
	"time"

	"goanalytics/services/worker/internal/domain/site"
)

// SiteCacheKey representa la identidad de cache para metadata de site.
//
// Incluye site_code, origin, environment y token_version porque esos datos
// determinan la resolucion y validacion posterior de un evento. Debe
// construirse desde application con datos ya extraidos del evento crudo.
type SiteCacheKey struct {
	SiteCode     string
	Origin       string
	Environment  string
	TokenVersion int
}

// SiteCache define el puerto para cachear metadata de site.
//
// Get recibe una clave de evento y devuelve SiteConfig, un booleano que indica
// si existe en cache y error ante fallas de infraestructura. Set recibe la
// misma clave, SiteConfig y TTL para guardar o renovar la metadata.
//
// Debe implementarse en adaptadores outbound, inicialmente Redis. La cache debe
// usar prefijos `goanalytics:*` y no exponer detalles al dominio.
type SiteCache interface {
	Get(ctx context.Context, key SiteCacheKey) (site.SiteConfig, bool, error)
	Set(ctx context.Context, key SiteCacheKey, site site.SiteConfig, ttl time.Duration) error
}
