package http

import (
	"context"
	"net/http"
	"time"
)

// ReadinessCheck valida si una dependencia operativa esta lista.
//
// Recibe el contexto del request y devuelve error cuando el servicio no puede
// aceptar trafico de forma segura. Se usa desde handlers HTTP operativos sin
// exponer detalles concretos de infraestructura.
type ReadinessCheck func(ctx context.Context) error

// NewHealthHandler crea un handler HTTP de liveness.
//
// No recibe parametros. Devuelve un handler que responde 200 cuando el proceso
// esta vivo. No valida dependencias externas; para eso debe usarse ready.
func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
}

// NewReadyHandler crea un handler HTTP de readiness.
//
// Recibe una funcion de verificacion de dependencias. Devuelve 200 cuando la
// verificacion finaliza sin error y 503 cuando alguna dependencia necesaria no
// esta disponible.
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
