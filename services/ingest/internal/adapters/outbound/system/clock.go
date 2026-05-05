package system

import "time"

// Clock obtiene la hora actual desde el reloj del sistema.
//
// No recibe entradas y devuelve un time.Time en UTC para que los eventos y
// validaciones usen una referencia consistente. No devuelve error porque el
// reloj local se considera una dependencia disponible del proceso.
type Clock struct{}

// Now devuelve la hora actual del sistema en UTC.
//
// No recibe parametros y retorna un time.Time normalizado a UTC. Debe usarse
// como implementacion concreta del puerto Clock en bootstrap.
func (Clock) Now() time.Time {
	return time.Now().UTC()
}
