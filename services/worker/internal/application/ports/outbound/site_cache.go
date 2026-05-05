package outbound

import (
	"context"
	"time"

	"goanalytics/services/worker/internal/domain/site"
)

// SiteCache define el puerto para cachear metadata de site.
//
// GetByPublicID recibe un identificador publico y devuelve SiteConfig, un
// booleano que indica si existe en cache y error ante fallas de infraestructura.
// Set recibe SiteConfig y TTL para guardar o renovar la metadata.
//
// Debe implementarse en adaptadores outbound, inicialmente Redis. La cache debe
// usar prefijos `goanalytics:*` y no exponer detalles al dominio.
type SiteCache interface {
	GetByPublicID(ctx context.Context, sitePublicID string) (site.SiteConfig, bool, error)
	Set(ctx context.Context, site site.SiteConfig, ttl time.Duration) error
}
