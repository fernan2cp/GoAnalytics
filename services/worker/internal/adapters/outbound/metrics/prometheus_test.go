package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryExportsCataloguedMetricsAndRejectsUnsafeLabels(t *testing.T) {
	registry := NewRegistry()
	registry.IncrementCounter("event_persistence_outcomes_total", map[string]string{"outcome": "inserted"})
	registry.IncrementCounter("event_persistence_outcomes_total", map[string]string{"tenant_id": "tenant-1"})
	registry.SetGauge("redis_pending_events_count", 7, nil)

	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `event_persistence_outcomes_total{outcome="inserted"} 1`) {
		t.Fatalf("contador esperado ausente:\n%s", body)
	}
	if !strings.Contains(body, "redis_pending_events_count 7") {
		t.Fatalf("gauge esperado ausente:\n%s", body)
	}
	if strings.Contains(body, "tenant-1") {
		t.Fatalf("label no permitida expuesta:\n%s", body)
	}
}

func TestRegistryAccumulatesHistogram(t *testing.T) {
	registry := NewRegistry()
	registry.ObserveHistogram("subject_bootstrap_duration_seconds", 2.5, nil)
	registry.ObserveHistogram("subject_bootstrap_duration_seconds", 1.5, nil)

	body := registry.render()
	if !strings.Contains(body, "subject_bootstrap_duration_seconds_count 2") ||
		!strings.Contains(body, "subject_bootstrap_duration_seconds_sum 4") {
		t.Fatalf("histograma inesperado:\n%s", body)
	}
}
