package outbound

const (
	MetricRedisPendingEvents  = "redis_pending_events_count"
	MetricRedisAutoclaim      = "redis_autoclaim_total"
	MetricRedisRetryExhausted = "redis_retry_exhausted_total"
	MetricEventPersistence    = "event_persistence_outcomes_total"
)

// Metrics define el puerto de observabilidad numerica del worker.
//
// Los nombres y labels aceptados forman parte de un catálogo acotado en el
// adaptador. Las implementaciones deben ignorar series desconocidas para que
// la observabilidad nunca interrumpa el procesamiento analítico.
type Metrics interface {
	IncrementCounter(name string, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
}
