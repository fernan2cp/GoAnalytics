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
