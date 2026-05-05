package outbound

import "time"

// Clock define el puerto para obtener tiempo actual.
//
// No recibe entradas y devuelve time.Time. Permite testear casos de uso con
// relojes falsos sin depender del reloj del sistema.
//
// No devuelve errores; si una implementacion necesitara una fuente externa de
// tiempo, debe encapsular esa condicion antes de exponer este puerto.
type Clock interface {
	Now() time.Time
}
