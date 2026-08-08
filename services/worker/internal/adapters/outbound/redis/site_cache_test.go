package redis

import (
	"testing"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/site"
)

func TestSiteCacheKeyPreservesLegacyWebKeyAndIsolatesServer(t *testing.T) {
	base := outbound.SiteCacheKey{
		SiteCode: "site-code", Origin: "https://example.com", Environment: "production", TokenVersion: 1,
	}
	legacy := siteCacheKey(base)
	base.Source = site.SourceWeb
	if explicitWeb := siteCacheKey(base); explicitWeb != legacy {
		t.Fatalf("clave web explicita = %q, want legacy %q", explicitWeb, legacy)
	}
	base.Source = site.SourceServer
	if server := siteCacheKey(base); server == legacy {
		t.Fatalf("clave server no deberia reutilizar clave web %q", legacy)
	}
}

func TestSitePayloadKeepsTimezoneOptionalAndRoundTripsIt(t *testing.T) {
	legacy := sitePayload{SiteCode: "legacy"}.toDomain()
	if legacy.Timezone != "" {
		t.Fatalf("Timezone legacy = %q", legacy.Timezone)
	}

	config := site.SiteConfig{SiteCode: "site", Timezone: "America/Argentina/Buenos_Aires"}
	if got := sitePayloadFromDomain(config).toDomain().Timezone; got != config.Timezone {
		t.Fatalf("Timezone = %q, want %q", got, config.Timezone)
	}
}
