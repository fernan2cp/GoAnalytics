package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	nethttp "net/http"
)

const requestIDHeader = "X-Request-Id"

// RequestIDMiddleware agrega un request_id a cada solicitud HTTP.
//
// Recibe el siguiente handler y devuelve un handler decorado que respeta
// X-Request-Id entrante o genera uno nuevo. El valor queda disponible en el
// contexto y en la respuesta.
func RequestIDMiddleware(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		requestID := request.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		writer.Header().Set(requestIDHeader, requestID)
		ctx := context.WithValue(request.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// CORSMiddleware gestiona las cabeceras de Cross-Origin Resource Sharing.
//
// Recibe los origenes permitidos y el siguiente handler. Si la solicitud es
// OPTIONS, responde directamente con las cabeceras de control. Para otras
// solicitudes, agrega la cabecera Access-Control-Allow-Origin correspondiente.
func CORSMiddleware(allowedOrigins []string, next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		origin := request.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(writer, request)
			return
		}

		allowed := false
		if len(allowedOrigins) > 0 && allowedOrigins[0] == "*" {
			allowed = true
			writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			for _, o := range allowedOrigins {
				if o == origin {
					allowed = true
					writer.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		if !allowed {
			next.ServeHTTP(writer, request)
			return
		}

		writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
		writer.Header().Set("Access-Control-Max-Age", "86400")

		if request.Method == nethttp.MethodOptions {
			writer.WriteHeader(nethttp.StatusNoContent)
			return
		}

		next.ServeHTTP(writer, request)
	})
}

// newRequestID genera un identificador hexadecimal para trazabilidad.
//
// No recibe parametros y devuelve una cadena de 32 caracteres. Si la entropia
// del sistema falla, usa un valor fijo de baja utilidad pero seguro para evitar
// panics en el middleware.
func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes[:])
}
