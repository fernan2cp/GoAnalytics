package httphealth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOperationalRouterSeparatesHealthReadyAndMetrics(t *testing.T) {
	router := NewRouterWithMetrics(
		NewHealthHandler(),
		NewReadyHandler(func(_ context.Context) error { return nil }),
		http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("metric 1\n"))
		}),
	)

	for _, path := range []string{"/health", "/ready", "/metrics"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, recorder.Code)
		}
	}
}
