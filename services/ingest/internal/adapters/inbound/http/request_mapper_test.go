package http

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeIngestRequestAcceptsSingleEventAliasesAndExtras(t *testing.T) {
	body := `{
		"event_id":"evt_1",
		"event_type":"page_view",
		"event_version":1,
		"timestamp":"2026-05-06T23:59:00Z",
		"origin":"http://body.example",
		"page_url":"http://body.example/productos",
		"path":"/productos",
		"metadata":{"source":"dragonfly"},
		"properties":{"source":"sdk"},
		"raw":{"ok":true},
		"items":[{"sku":"a"}],
		"filters":{"q":"zapato"},
		"consent":{"analytics":true},
		"app":{"name":"dragonfly"},
		"route_name":"productos",
		"search_term":"zapato",
		"previous_page":"/"
	}`
	request := httptest.NewRequest("POST", "/v1/events", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer token_123")
	request.Header.Set("Origin", "http://header.example")

	decoded, err := decodeIngestRequest(request)
	if err != nil {
		t.Fatalf("decodeIngestRequest() error = %v", err)
	}
	if len(decoded.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(decoded.Events))
	}

	event := decoded.Events[0]
	if event.EventName != "page_view" {
		t.Fatalf("EventName = %q, want page_view", event.EventName)
	}
	if event.URL != "http://body.example/productos" {
		t.Fatalf("URL = %q, want page_url alias", event.URL)
	}
	if event.Origin != "http://header.example" {
		t.Fatalf("Origin = %q, want header origin", event.Origin)
	}
	if event.Properties["source"] != "sdk" {
		t.Fatalf("properties source = %#v, want sdk", event.Properties["source"])
	}
	if _, ok := event.Properties["raw"]; !ok {
		t.Fatalf("raw extra no fue preservado en properties")
	}
	if _, ok := event.Properties["items"]; !ok {
		t.Fatalf("items extra no fue preservado en properties")
	}
	if event.Properties["route_name"] != "productos" {
		t.Fatalf("route_name = %#v, want productos", event.Properties["route_name"])
	}
	if _, ok := event.Context["consent"]; !ok {
		t.Fatalf("consent extra no fue preservado en context")
	}
	if _, ok := event.Context["app"]; !ok {
		t.Fatalf("app extra no fue preservado en context")
	}
}

func TestDecodeIngestRequestAcceptsBatchWithUnknownFields(t *testing.T) {
	body := `{
		"events":[{
			"event_id":"evt_1",
			"event_name":"page_view",
			"event_version":1,
			"timestamp":"2026-05-06T23:59:00Z",
			"anonymous_id":"anon",
			"origin":"http://127.0.0.1:5173",
			"url":"http://127.0.0.1:5173/",
			"path":"/",
			"unknown_future_field":true
		}]
	}`
	request := httptest.NewRequest("POST", "/v1/events", strings.NewReader(body))

	decoded, err := decodeIngestRequest(request)
	if err != nil {
		t.Fatalf("decodeIngestRequest() error = %v", err)
	}
	if len(decoded.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(decoded.Events))
	}
}

func TestDecodeIngestRequestPreservesSiteMetadataInContext(t *testing.T) {
	body := `{
		"event_id":"evt_1",
		"event_name":"page_view",
		"event_version":1,
		"timestamp":"2026-05-06T23:59:00Z",
		"anonymous_id":"anon",
		"origin":"http://127.0.0.1:5173",
		"url":"http://127.0.0.1:5173/",
		"path":"/",
		"site_code":"df_devel_0_temp",
		"tenant_id":"demo_eshop_erp_20260225_223010",
		"site_id":"1",
		"status":"active",
		"tracking_enabled":true,
		"allowed_domains":["127.0.0.1"]
	}`
	request := httptest.NewRequest("POST", "/v1/events", strings.NewReader(body))

	decoded, err := decodeIngestRequest(request)
	if err != nil {
		t.Fatalf("decodeIngestRequest() error = %v", err)
	}
	siteContext, ok := decoded.Events[0].Context["site"].(map[string]any)
	if !ok {
		t.Fatalf("context.site = %#v, want map", decoded.Events[0].Context["site"])
	}
	if siteContext["site_code"] != "df_devel_0_temp" {
		t.Fatalf("site_code = %#v, want df_devel_0_temp", siteContext["site_code"])
	}
	if siteContext["tracking_enabled"] != true {
		t.Fatalf("tracking_enabled = %#v, want true", siteContext["tracking_enabled"])
	}
	if _, ok := siteContext["allowed_domains"]; !ok {
		t.Fatalf("allowed_domains no fue preservado en context.site")
	}
}
