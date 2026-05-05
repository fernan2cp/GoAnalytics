package site

// SiteConfig representa la metadata real de un site para validacion.
//
// Contiene identidad interna, estado, dominios permitidos, version de token y
// parametros de muestreo. Se usa como tipo de dominio obtenido desde cache o
// resolver interno.
//
// Debe provenir de Redis o del SiteResolver y estar alineado con el contrato
// documentado. No devuelve errores por si mismo; las fallas aparecen al
// obtenerlo o validarlo.
type SiteConfig struct {
	SitePublicID    string
	TenantID        string
	SiteID          string
	Status          string
	TrackingEnabled bool
	AllowedDomains  []string
	TokenVersion    int
	SampleRate      float64
	SchemaVersion   int
}
