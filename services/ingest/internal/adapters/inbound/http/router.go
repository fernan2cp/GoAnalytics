package http

import nethttp "net/http"

// NewRouter crea el router HTTP de la API de ingesta.
//
// Recibe el handler de eventos y devuelve un http.Handler con rutas y
// middleware iniciales. Mantiene net/http para evitar dependencias de framework
// en Fase 2.
func NewRouter(eventsHandler *IngestHandler) nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.Handle("/v1/events", eventsHandler)
	return RequestIDMiddleware(mux)
}
