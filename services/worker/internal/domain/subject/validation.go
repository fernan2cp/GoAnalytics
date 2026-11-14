package subject

import (
	"errors"
	"strings"
)

var (
	// ErrInvalidIdentity indica que faltan componentes de la identidad canónica.
	ErrInvalidIdentity = errors.New("identidad de sujeto inválida")
	// ErrUnsupportedType indica un tipo de sujeto no habilitado en el MVP.
	ErrUnsupportedType = errors.New("tipo de sujeto no soportado")
	// ErrInvalidLifecycle indica una combinación de estado y merge inconsistente.
	ErrInvalidLifecycle = errors.New("lifecycle de sujeto inválido")
)

// ValidateIdentity valida la terna canónica tenant, tipo e ID externo.
func ValidateIdentity(tenantID string, subjectType Type, externalSubjectID string) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(externalSubjectID) == "" {
		return ErrInvalidIdentity
	}
	if subjectType != CustomerType {
		return ErrUnsupportedType
	}
	return nil
}

// ValidateLifecycle comprueba estados admitidos y coherencia del destino merge.
func ValidateLifecycle(status LifecycleStatus, subjectID int64, mergedIntoSubjectID *int64) error {
	if status != LifecycleUnknown && status != LifecycleActive && status != LifecycleInactive && status != LifecycleMerged {
		return ErrInvalidLifecycle
	}
	if status == LifecycleMerged {
		if mergedIntoSubjectID == nil || *mergedIntoSubjectID == subjectID {
			return ErrInvalidLifecycle
		}
		return nil
	}
	if mergedIntoSubjectID != nil {
		return ErrInvalidLifecycle
	}
	return nil
}
