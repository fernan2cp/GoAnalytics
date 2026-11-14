package dto

// StreamRejection describe un mensaje de Redis Stream que no puede continuar.
//
// Recibe identificadores recuperables, el motivo estable, el payload de
// transporte y su cantidad de entregas. La capa de aplicacion usa estos datos
// para construir una auditoria segura sin persistir el contenido crudo.
//
// No valida el motivo ni accede a infraestructura. Los campos de identidad
// pueden estar vacios cuando el payload no es JSON valido.
type StreamRejection struct {
	StreamID      string
	EventID       string
	SiteCode      string
	Environment   string
	Reason        string
	Payload       string
	DeliveryCount int64
}
