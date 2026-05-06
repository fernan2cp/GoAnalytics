package httphealth

import (
	"context"
	"net/http"
	"time"
)

// ReadinessCheck valida si las dependencias operativas del worker estan listas.
//
// Recibe el contexto del request y devuelve error cuando Redis, PostgreSQL u
// otra dependencia critica no esta disponible.
type ReadinessCheck func(ctx context.Context) error

// NewHealthHandler crea un handler HTTP de liveness del worker.
//
// No recibe parametros. Devuelve un handler que responde 200 cuando el proceso
// esta vivo. No valida dependencias externas.
func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
}

// NewReadyHandler crea un handler HTTP de readiness del worker.
//
// Recibe una funcion de verificacion de dependencias. Devuelve 200 cuando el
// worker puede operar y 503 cuando alguna dependencia critica no responde.
func NewReadyHandler(check ReadinessCheck) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()

		if check != nil {
			if err := check(ctx); err != nil {
				http.Error(writer, "servicio no listo", http.StatusServiceUnavailable)
				return
			}
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ready"}`))
	})
}

// NewRouter crea el router HTTP operativo del worker.
//
// Recibe handlers de liveness y readiness. Devuelve un http.Handler aislado de
// la API de negocio; solo debe exponerse para healthchecks locales.
func NewRouter(healthHandler http.Handler, readyHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler)
	mux.Handle("/ready", readyHandler)
	return mux
}
