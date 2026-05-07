package dto

// IngestResponse representa la salida del caso de uso de ingesta.
//
// Expone la cantidad de eventos aceptados y rechazados, junto con los IDs que
// fueron publicados para procesamiento. Se usa como DTO de aplicacion para que
// el adaptador HTTP decida la respuesta externa.
//
// No contiene errores de infraestructura ni de contrato global; esas fallas se
// devuelven por el valor error del caso de uso.
type IngestResponse struct {
	Accepted int      `json:"accepted"`
	Rejected int      `json:"rejected"`
	EventIDs []string `json:"event_ids"`
}
