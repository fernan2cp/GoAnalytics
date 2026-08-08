package metrics

// Noop descarta observaciones sin alterar el flujo llamador.
//
// Permite ejecutar casos de uso y pruebas sin un exporter configurado.
type Noop struct{}

// IncrementCounter descarta un incremento de contador.
func (Noop) IncrementCounter(_ string, _ map[string]string) {}

// SetGauge descarta una actualización de gauge.
func (Noop) SetGauge(_ string, _ float64, _ map[string]string) {}

// ObserveHistogram descarta una observación de histograma.
func (Noop) ObserveHistogram(_ string, _ float64, _ map[string]string) {}
