package dto

// IngestResponse representa la salida del caso de uso de ingesta.
//
// Expone la cantidad de eventos aceptados para procesamiento. Se usa como DTO
// de aplicacion para que el adaptador HTTP decida la respuesta externa.
//
// No contiene errores; las fallas se devuelven por el valor error del caso de
// uso.
type IngestResponse struct {
	Accepted int
}
