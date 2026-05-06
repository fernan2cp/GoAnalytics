package http

import nethttp "net/http"

// NewRouter crea el router HTTP de la API de ingesta.
//
// Recibe handlers de eventos, liveness y readiness, y devuelve un http.Handler
// con rutas y middleware iniciales. Mantiene net/http para evitar dependencias
// de framework.
func NewRouter(eventsHandler *IngestHandler, healthHandler nethttp.Handler, readyHandler nethttp.Handler) nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.Handle("/v1/events", eventsHandler)
	mux.Handle("/health", healthHandler)
	mux.Handle("/ready", readyHandler)
	return RequestIDMiddleware(mux)
}
