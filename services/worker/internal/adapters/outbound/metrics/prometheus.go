package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type metricKind string

const (
	counterKind   metricKind = "counter"
	gaugeKind     metricKind = "gauge"
	histogramKind metricKind = "histogram"
)

type metricDefinition struct {
	Help          string
	Kind          metricKind
	AllowedLabels map[string]map[string]struct{}
}

var subjectMetricDefinitions = map[string]metricDefinition{
	"redis_pending_events_count":  {Help: "Mensajes pendientes en el consumer group.", Kind: gaugeKind},
	"redis_autoclaim_total":       {Help: "Ejecuciones exitosas de XAUTOCLAIM.", Kind: counterKind},
	"redis_retry_exhausted_total": {Help: "Mensajes rechazados por agotar entregas.", Kind: counterKind},
	"event_persistence_outcomes_total": {
		Help: "Resultados durables de persistencia por evento.", Kind: counterKind,
		AllowedLabels: labelValues("outcome", "inserted", "duplicate_exact", "duplicate_logical", "duplicate_idempotency", "conflict", "error", "invalid_result", "unknown"),
	},
	"subject_normalization_total":            {Help: "Resultados de normalización de sujetos.", Kind: counterKind, AllowedLabels: unrestrictedLabels("status", "event_name")},
	"subject_lifecycle_ignored_total":        {Help: "Lifecycle ignorado por precedencia.", Kind: counterKind, AllowedLabels: unrestrictedLabels("reason")},
	"subject_bootstrap_pages_total":          {Help: "Páginas procesadas por bootstrap.", Kind: counterKind, AllowedLabels: unrestrictedLabels("status")},
	"subject_bootstrap_subjects_total":       {Help: "Sujetos procesados por bootstrap.", Kind: counterKind, AllowedLabels: unrestrictedLabels("status")},
	"subject_bootstrap_duration_seconds":     {Help: "Duración del bootstrap.", Kind: histogramKind},
	"subject_projection_runs_total":          {Help: "Ejecuciones de proyección.", Kind: counterKind, AllowedLabels: unrestrictedLabels("projection", "status")},
	"subject_projection_duration_seconds":    {Help: "Duración de proyecciones.", Kind: histogramKind, AllowedLabels: unrestrictedLabels("projection")},
	"subject_projection_lag_seconds":         {Help: "Antigüedad de la proyección disponible.", Kind: gaugeKind, AllowedLabels: unrestrictedLabels("projection")},
	"subject_projection_rows_total":          {Help: "Filas procesadas por proyección.", Kind: counterKind, AllowedLabels: unrestrictedLabels("projection", "operation")},
	"subject_context_registry_loads_total":   {Help: "Cargas del registro de contextos.", Kind: counterKind, AllowedLabels: unrestrictedLabels("status")},
	"subject_context_registry_revision_info": {Help: "Revisión activa del registro.", Kind: gaugeKind, AllowedLabels: unrestrictedLabels("revision")},
	"subject_unregistered_contexts_total":    {Help: "Contextos no registrados observados.", Kind: counterKind},
	"subject_query_requests_total":           {Help: "Consultas de sugerencias.", Kind: counterKind, AllowedLabels: unrestrictedLabels("status", "source_level")},
	"subject_query_duration_seconds":         {Help: "Duración de consultas de sugerencias.", Kind: histogramKind, AllowedLabels: unrestrictedLabels("status")},
	"subject_query_stale_total":              {Help: "Respuestas servidas con proyección vencida.", Kind: counterKind},
}

// Registry mantiene series acotadas y expone el formato de texto Prometheus.
//
// Rechaza nombres o labels fuera del catálogo para impedir cardinalidad no
// controlada y fuga accidental de tenant, eventos o sujetos.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]float64
	gauges     map[string]float64
	histograms map[string]histogramValue
}

type histogramValue struct {
	Count   uint64
	Sum     float64
	Buckets []uint64
}

var defaultHistogramBuckets = []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.3, 0.5, 1, 2.5, 5, 10, 30, 60, 300, 1800, 3600}

// NewRegistry crea un registro Prometheus vacío y seguro para concurrencia.
func NewRegistry() *Registry {
	return &Registry{
		counters: make(map[string]float64), gauges: make(map[string]float64),
		histograms: make(map[string]histogramValue),
	}
}

// IncrementCounter incrementa una serie conocida en una unidad.
func (registry *Registry) IncrementCounter(name string, labels map[string]string) {
	key, ok := validatedSeries(name, counterKind, labels)
	if registry == nil || !ok {
		return
	}
	registry.mu.Lock()
	registry.counters[key]++
	registry.mu.Unlock()
}

// SetGauge reemplaza el valor de una serie gauge conocida.
func (registry *Registry) SetGauge(name string, value float64, labels map[string]string) {
	key, ok := validatedSeries(name, gaugeKind, labels)
	if registry == nil || !ok {
		return
	}
	registry.mu.Lock()
	registry.gauges[key] = value
	registry.mu.Unlock()
}

// ObserveHistogram acumula cantidad y suma para una serie conocida.
func (registry *Registry) ObserveHistogram(name string, value float64, labels map[string]string) {
	key, ok := validatedSeries(name, histogramKind, labels)
	if registry == nil || !ok || value < 0 {
		return
	}
	registry.mu.Lock()
	current := registry.histograms[key]
	if current.Buckets == nil {
		current.Buckets = make([]uint64, len(defaultHistogramBuckets))
	}
	current.Count++
	current.Sum += value
	for index, upperBound := range defaultHistogramBuckets {
		if value <= upperBound {
			current.Buckets[index]++
		}
	}
	registry.histograms[key] = current
	registry.mu.Unlock()
}

// Handler devuelve el endpoint de exposición Prometheus del registro.
func (registry *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(registry.render()))
	})
}

func (registry *Registry) render() string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	var builder strings.Builder
	names := make([]string, 0, len(subjectMetricDefinitions))
	for name := range subjectMetricDefinitions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		definition := subjectMetricDefinitions[name]
		fmt.Fprintf(&builder, "# HELP %s %s\n# TYPE %s %s\n", name, definition.Help, name, definition.Kind)
		writeSeries(&builder, name, registry.counters)
		writeSeries(&builder, name, registry.gauges)
		writeHistograms(&builder, name, registry.histograms)
	}
	return builder.String()
}

func writeSeries(builder *strings.Builder, name string, values map[string]float64) {
	keys := matchingSeries(name, values)
	for _, key := range keys {
		fmt.Fprintf(builder, "%s %s\n", key, strconv.FormatFloat(values[key], 'g', -1, 64))
	}
}

func writeHistograms(builder *strings.Builder, name string, values map[string]histogramValue) {
	keys := matchingSeries(name, values)
	for _, key := range keys {
		value := values[key]
		for index, upperBound := range defaultHistogramBuckets {
			fmt.Fprintf(builder, "%s %d\n", histogramBucketKey(key, strconv.FormatFloat(upperBound, 'g', -1, 64)), value.Buckets[index])
		}
		fmt.Fprintf(builder, "%s %d\n", histogramBucketKey(key, "+Inf"), value.Count)
		fmt.Fprintf(builder, "%s %d\n%s %s\n", histogramSeriesKey(key, "count"), value.Count, histogramSeriesKey(key, "sum"), strconv.FormatFloat(value.Sum, 'g', -1, 64))
	}
}

func histogramBucketKey(key string, upperBound string) string {
	name, labels, found := strings.Cut(key, "{")
	if !found {
		return name + `_bucket{le="` + upperBound + `"}`
	}
	labels = strings.TrimSuffix(labels, "}")
	return name + `_bucket{` + labels + `,le="` + upperBound + `"}`
}

func histogramSeriesKey(key string, suffix string) string {
	name, labels, found := strings.Cut(key, "{")
	if !found {
		return name + "_" + suffix
	}
	return name + "_" + suffix + "{" + labels
}

func matchingSeries[T any](name string, values map[string]T) []string {
	keys := make([]string, 0)
	for key := range values {
		if key == name || strings.HasPrefix(key, name+"{") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func validatedSeries(name string, kind metricKind, labels map[string]string) (string, bool) {
	definition, ok := subjectMetricDefinitions[name]
	if !ok || definition.Kind != kind || len(labels) != len(definition.AllowedLabels) {
		return "", false
	}
	keys := make([]string, 0, len(labels))
	for key, value := range labels {
		allowedValues, allowed := definition.AllowedLabels[key]
		if !allowed || strings.TrimSpace(value) == "" {
			return "", false
		}
		if len(allowedValues) > 0 {
			if _, allowed = allowedValues[value]; !allowed {
				return "", false
			}
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return name, true
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s=%q`, key, labels[key]))
	}
	return name + "{" + strings.Join(parts, ",") + "}", true
}

func labelValues(name string, values ...string) map[string]map[string]struct{} {
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		allowed[value] = struct{}{}
	}
	return map[string]map[string]struct{}{name: allowed}
}

func unrestrictedLabels(names ...string) map[string]map[string]struct{} {
	labels := make(map[string]map[string]struct{}, len(names))
	for _, name := range names {
		labels[name] = nil
	}
	return labels
}
