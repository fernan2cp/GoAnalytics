package system

import "time"

// Clock implementa el puerto de tiempo usando el reloj del sistema.
//
// No contiene estado ni requiere configuracion. Se inyecta desde bootstrap
// para que application no dependa directamente de time.Now.
type Clock struct{}

// Now devuelve el instante actual en UTC.
//
// No recibe parametros y devuelve time.Time normalizado a UTC. No devuelve
// errores porque el reloj del sistema siempre produce un valor.
func (Clock) Now() time.Time {
	return time.Now().UTC()
}
