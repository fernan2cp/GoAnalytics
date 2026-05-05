package redis

import "errors"

// Errores estables de los adaptadores Redis del worker.
//
// Permiten distinguir negative cache y cooldown sin filtrar detalles de claves
// o comandos Redis hacia las capas superiores.
var (
	// ErrNegativeCached indica que existe una marca temporal de site ausente.
	ErrNegativeCached = errors.New("site marcado como no encontrado en cache")

	// ErrRehydrateCooldown indica que ya hubo un intento reciente de resolver el site.
	ErrRehydrateCooldown = errors.New("rehidratacion en cooldown")
)
