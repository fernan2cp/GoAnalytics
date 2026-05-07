package httpresolver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

func TestSiteResolverAcceptsMinimalSiteContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input resolveRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatalf("request decode error = %v", err)
		}
		if input.TokenVersion != 7 {
			t.Fatalf("TokenVersion = %d, want 7", input.TokenVersion)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"site_code":"df_devel_0_temp",
			"tenant_id":"demo_eshop_erp_20260225_223010",
			"site_id":"1",
			"status":"active",
			"tracking_enabled":true,
			"allowed_domains":["127.0.0.1"]
		}`))
	}))
	defer server.Close()

	resolver, err := NewSiteResolver(server.URL, "", time.Second, nil, nil)
	if err != nil {
		t.Fatalf("NewSiteResolver() error = %v", err)
	}

	config, err := resolver.Resolve(context.Background(), outbound.ResolveSiteInput{
		SiteCode:     "df_devel_0_temp",
		Origin:       "http://127.0.0.1:5173",
		Environment:  "development",
		TokenVersion: 7,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if config.TokenVersion != 7 {
		t.Fatalf("TokenVersion = %d, want 7", config.TokenVersion)
	}
	if config.SampleRate != 1 {
		t.Fatalf("SampleRate = %v, want 1", config.SampleRate)
	}
	if config.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", config.SchemaVersion)
	}
}
