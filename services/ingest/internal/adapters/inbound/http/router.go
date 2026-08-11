package http

import nethttp "net/http"

// NewRouter crea el router HTTP de la API de ingesta.
//
// Recibe handlers de eventos, liveness y readiness, junto con los origenes
// permitidos para CORS, y devuelve un http.Handler decorado con middleware.
func NewRouter(eventsHandler *IngestHandler, healthHandler nethttp.Handler, readyHandler nethttp.Handler, allowedOrigins []string) nethttp.Handler {
	return NewRouterWithInternal(eventsHandler, nil, healthHandler, readyHandler, allowedOrigins)
}

// NewRouterWithInternal crea el router y registra opcionalmente la capacidad interna.
//
// Un internalEventsHandler nulo mantiene la ruta sin registrar y produce 404,
// preservando el comportamiento opt-in. Los demás argumentos conservan el
// mismo contrato de NewRouter.
func NewRouterWithInternal(eventsHandler *IngestHandler, internalEventsHandler *InternalIngestHandler, healthHandler nethttp.Handler, readyHandler nethttp.Handler, allowedOrigins []string) nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.Handle("/v1/events", eventsHandler)
	if internalEventsHandler != nil {
		mux.Handle("/v1/internal/events", internalEventsHandler)
	}
	mux.Handle("/health", healthHandler)
	mux.Handle("/ready", readyHandler)

	handler := CORSMiddleware(allowedOrigins, mux)
	return RequestIDMiddleware(handler)
}
