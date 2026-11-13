package subject

import "errors"

// ErrBootstrapCompleted indica que el corte previo ya se importó completamente.
var ErrBootstrapCompleted = errors.New("bootstrap completado")

// ErrBootstrapExpired exige iniciar explícitamente un corte nuevo.
var ErrBootstrapExpired = errors.New("snapshot_revision_expired")
