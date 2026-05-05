package outbound

import "time"

// Clock define el puerto para obtener tiempo actual.
//
// No recibe entradas y devuelve time.Time. Permite testear procesamiento,
// rechazos y TTLs con relojes falsos sin depender del reloj del sistema.
//
// No devuelve errores; las implementaciones deben ser deterministas para los
// casos de uso.
type Clock interface {
	Now() time.Time
}
