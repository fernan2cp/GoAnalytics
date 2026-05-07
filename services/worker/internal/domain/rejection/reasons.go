package rejection

// Motivos y severidades estables para registrar eventos rechazados.
//
// Se definen en dominio para que application y futuros adaptadores compartan
// valores consistentes sin depender de literales dispersos.
const (
	ReasonInvalidPayload         = "invalid_payload"
	ReasonSiteUnresolved         = "site_unresolved"
	ReasonInvalidSiteConfig      = "invalid_site_config"
	ReasonSiteInactive           = "site_inactive"
	ReasonTrackingDisabled       = "tracking_disabled"
	ReasonTokenVersion           = "token_version_mismatch"
	ReasonDomainNotAllowed       = "domain_not_allowed"
	ReasonDuplicateEvent         = "duplicate_event"
	ReasonDuplicateLogicalEvent  = "duplicate_logical_event"
	ReasonDuplicateSemanticEvent = "duplicate_semantic_event"
	ReasonSensitivePayload       = "sensitive_payload"
	ReasonPayloadTooLarge        = "payload_too_large"
	ReasonBlockedKey             = "blocked_key"

	SeverityWarning    = "warning"
	SeveritySuspicious = "suspicious"
)
