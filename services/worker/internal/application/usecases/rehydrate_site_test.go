package usecases

import (
	"context"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/site"
)

func TestRehydrateSiteAllowsServerWithoutOriginOrTrackingToken(t *testing.T) {
	resolver := &rehydrateSiteResolverFake{config: validRehydratedSite()}
	cache := &rehydrateSiteCacheFake{}
	useCase := NewRehydrateSiteUseCase(resolver, cache, RehydrateSiteOptions{SiteCacheTTL: time.Minute})

	_, err := useCase.Rehydrate(context.Background(), outbound.SiteCacheKey{
		SiteCode: "site-code", Environment: "production", Source: site.SourceServer,
	})
	if err != nil {
		t.Fatalf("Rehydrate() error = %v", err)
	}
	if resolver.input.Source != site.SourceServer || resolver.input.Origin != "" || resolver.input.TokenVersion != 0 {
		t.Fatalf("entrada resolver inesperada: %#v", resolver.input)
	}
}

func TestRehydrateSiteKeepsLegacyWebRequirements(t *testing.T) {
	useCase := NewRehydrateSiteUseCase(
		&rehydrateSiteResolverFake{config: validRehydratedSite()},
		&rehydrateSiteCacheFake{},
		RehydrateSiteOptions{},
	)

	if _, err := useCase.Rehydrate(context.Background(), outbound.SiteCacheKey{
		SiteCode: "site-code", Environment: "production",
	}); err != ErrRehydrateInputInvalid {
		t.Fatalf("Rehydrate() error = %v, want ErrRehydrateInputInvalid", err)
	}
}

type rehydrateSiteResolverFake struct {
	config site.SiteConfig
	input  outbound.ResolveSiteInput
}

func (fake *rehydrateSiteResolverFake) Resolve(_ context.Context, input outbound.ResolveSiteInput) (site.SiteConfig, error) {
	fake.input = input
	return fake.config, nil
}

type rehydrateSiteCacheFake struct{}

func (fake *rehydrateSiteCacheFake) Get(_ context.Context, _ outbound.SiteCacheKey) (site.SiteConfig, bool, error) {
	return site.SiteConfig{}, false, nil
}

func (fake *rehydrateSiteCacheFake) Set(_ context.Context, _ outbound.SiteCacheKey, _ site.SiteConfig, _ time.Duration) error {
	return nil
}

func validRehydratedSite() site.SiteConfig {
	return site.SiteConfig{
		SiteCode: "site-code", TenantID: "tenant-1", SiteID: "site-1", Status: "active",
		TrackingEnabled: true, AllowedDomains: []string{"example.com"}, TokenVersion: 1,
		SampleRate: 1, SchemaVersion: 2, Timezone: "America/Argentina/Buenos_Aires",
	}
}
