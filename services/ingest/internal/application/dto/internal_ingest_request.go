package dto

import "goanalytics/services/ingest/internal/domain/event"

// InternalIngestRequest representa un batch autenticado server-to-server.
//
// Events contiene perfiles canónicos sin campos de sesión ni navegación. La
// autenticación se resuelve en el adaptador HTTP antes de crear este DTO.
type InternalIngestRequest struct {
	Events []event.InternalEvent
}
