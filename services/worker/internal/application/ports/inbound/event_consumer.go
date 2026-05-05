package inbound

import "context"

// EventConsumer define el puerto inbound para ejecutar el worker.
//
// Recibe un contexto de ciclo de vida y devuelve error si el proceso de
// consumo no puede iniciar o termina por una falla no recuperable.
//
// Debe implementarse en un adaptador inbound, por ejemplo Redis Stream. La
// aplicacion usa casos de uso para procesar cada batch consumido.
type EventConsumer interface {
	Run(ctx context.Context) error
}
