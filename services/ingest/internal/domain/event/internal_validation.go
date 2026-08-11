package event

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidInternalEvent indica un perfil server incompleto o ambiguo.
	ErrInvalidInternalEvent = errors.New("evento interno inválido")
	// ErrUnsupportedInternalEvent indica un nombre fuera del catálogo de sujetos.
	ErrUnsupportedInternalEvent = errors.New("evento interno no soportado")
	// ErrUnsupportedSubjectType indica un tipo de sujeto no habilitado.
	ErrUnsupportedSubjectType = errors.New("tipo de sujeto no soportado")
)

const maxInternalFieldLength = 255

var supportedInternalEvents = map[string]struct{}{
	"subject_impression":  {},
	"subject_viewed":      {},
	"subject_selected":    {},
	"subject_associated":  {},
	"subject_deactivated": {},
	"subject_reactivated": {},
	"subject_merged":      {},
}

// ValidateInternalEvent valida el perfil canónico server-to-server.
//
// Recibe un InternalEvent ya decodificado y devuelve errores estables cuando
// faltan campos, el catálogo no contiene el evento o el tipo no es customer.
// No accede a infraestructura ni valida la existencia real del site.
func ValidateInternalEvent(input InternalEvent) error {
	if strings.TrimSpace(input.EventID) == "" || strings.TrimSpace(input.SiteCode) == "" || input.EventVersion != 1 || input.Timestamp.IsZero() {
		return ErrInvalidInternalEvent
	}
	if _, offset := input.Timestamp.Zone(); offset != 0 {
		return ErrInvalidInternalEvent
	}
	if _, ok := supportedInternalEvents[input.EventName]; !ok {
		return ErrUnsupportedInternalEvent
	}
	if len(input.Subjects) == 0 || len(input.Subjects) > 100 {
		return ErrInvalidInternalEvent
	}
	for _, subject := range input.Subjects {
		if err := validateSubjectReference(subject); err != nil {
			return err
		}
	}
	if input.Operation != nil {
		if !validRequiredText(input.Operation.OperationType) || !validRequiredText(input.Operation.OperationID) {
			return ErrInvalidInternalEvent
		}
	}

	isLifecycle := input.EventName == "subject_deactivated" || input.EventName == "subject_reactivated" || input.EventName == "subject_merged"
	requiresIdempotency := isLifecycle || input.EventName == "subject_associated"
	if requiresIdempotency && !validRequiredText(input.IdempotencyKey) {
		return ErrInvalidInternalEvent
	}
	if isLifecycle {
		return validateLifecycle(input.EventName, input.Lifecycle)
	}
	if input.Lifecycle != nil {
		return ErrInvalidInternalEvent
	}
	return nil
}

// validateSubjectReference valida identidad, relación y longitudes del sujeto.
func validateSubjectReference(subject SubjectReference) error {
	if subject.SubjectType != "customer" {
		return ErrUnsupportedSubjectType
	}
	if !validRequiredText(subject.SubjectID) || utf8.RuneCountInString(subject.Relation) > maxInternalFieldLength {
		return ErrInvalidInternalEvent
	}
	return nil
}

// validateLifecycle valida la tupla temporal y el destino exclusivo de merge.
func validateLifecycle(eventName string, lifecycle *LifecycleChange) error {
	if lifecycle == nil || lifecycle.SourceUpdatedAt.IsZero() || !validRequiredText(lifecycle.SourceMutationID) {
		return ErrInvalidInternalEvent
	}
	if _, offset := lifecycle.SourceUpdatedAt.Zone(); offset != 0 {
		return ErrInvalidInternalEvent
	}
	if lifecycle.EffectiveAt != nil {
		if _, offset := lifecycle.EffectiveAt.Zone(); offset != 0 {
			return ErrInvalidInternalEvent
		}
	}
	if eventName == "subject_merged" {
		if !validRequiredText(lifecycle.MergedIntoSubjectID) {
			return ErrInvalidInternalEvent
		}
		return nil
	}
	if strings.TrimSpace(lifecycle.MergedIntoSubjectID) != "" {
		return ErrInvalidInternalEvent
	}
	return nil
}

// validRequiredText valida contenido no vacío y longitud contractual.
func validRequiredText(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && utf8.RuneCountInString(trimmed) <= maxInternalFieldLength
}
