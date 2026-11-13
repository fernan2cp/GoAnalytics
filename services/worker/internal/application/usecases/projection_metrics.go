package usecases

import (
	"goanalytics/services/worker/internal/application/ports/outbound"
	"time"
)

// observeProjectionRun publica duración y resultado sin etiquetas de negocio.
func observeProjectionRun(metrics outbound.Metrics, projection, status string, started time.Time, err error) {
	if metrics == nil {
		return
	}
	if err != nil {
		status = "failed"
	}
	metrics.IncrementCounter("subject_projection_runs_total", map[string]string{"projection": projection, "status": status})
	metrics.ObserveHistogram("subject_projection_duration_seconds", time.Since(started).Seconds(), map[string]string{"projection": projection})
}
