package usecases

import (
	"encoding/json"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/subject"
)

const maxSubjectFieldLength = 255

// normalizeSubjectDetails transforma properties.subjects en detalles 1:N.
//
// Devuelve cero filas cuando la clave no existe. Cada elemento semánticamente
// inválido genera una fila auditable sin impedir que otros sujetos se
// normalicen. Los duplicados exactos conservan solo la primera posición.
func normalizeSubjectDetails(valid event.ValidatedEvent) []subject.EventDetail {
	rawSubjects, exists := valid.Properties["subjects"]
	if !exists {
		return nil
	}
	subjects, validList := subjectList(rawSubjects)
	if !validList {
		return []subject.EventDetail{invalidSubjectDetail(0, "subjects_not_array")}
	}
	operationType, operationID, operationValid := parseOperation(valid.Properties["operation"])
	lifecycleUpdatedAt, lifecycleMutationID, lifecycleEffectiveAt, mergedIntoID, lifecycleValid := parseLifecycle(valid.EventName, valid.Properties["lifecycle"])
	contextKey, contextVersion, claimedFamily := parseSubjectContext(valid.Context)
	localDate, timezoneValid := resolveLocalDate(valid.EventTime, valid.Timezone)

	details := make([]subject.EventDetail, 0, len(subjects))
	seen := make(map[string]struct{}, len(subjects))
	for ordinal, rawSubject := range subjects {
		values, ok := rawSubject.(map[string]any)
		if !ok {
			details = append(details, invalidSubjectDetail(ordinal, "subject_not_object"))
			continue
		}
		subjectType, _ := values["subject_type"].(string)
		externalID, _ := values["subject_id"].(string)
		relation, _ := values["relation"].(string)
		subjectType = strings.TrimSpace(subjectType)
		externalID = strings.TrimSpace(externalID)
		relation = strings.TrimSpace(relation)
		key := subjectType + "\x00" + externalID + "\x00" + relation
		if _, duplicated := seen[key]; duplicated {
			continue
		}
		seen[key] = struct{}{}

		detail := subject.EventDetail{
			SubjectOrdinal:              ordinal,
			SubjectType:                 subjectType,
			ExternalSubjectID:           externalID,
			Relation:                    relation,
			OperationType:               operationType,
			OperationID:                 operationID,
			ContextKey:                  contextKey,
			ContextVersion:              contextVersion,
			ClaimedContextFamily:        claimedFamily,
			ContextResolutionStatus:     subject.ContextRegistryDisabled,
			TimezoneName:                valid.Timezone,
			LocalDate:                   localDate,
			NormalizationStatus:         subject.NormalizationNormalized,
			Metadata:                    map[string]any{},
			InitialLifecycleStatus:      initialSubjectStatus(valid),
			LifecycleSourceUpdatedAt:    lifecycleUpdatedAt,
			LifecycleSourceMutationID:   lifecycleMutationID,
			LifecycleEffectiveAt:        lifecycleEffectiveAt,
			MergedIntoExternalSubjectID: mergedIntoID,
		}
		switch {
		case subjectType == "" || externalID == "" || utf8.RuneCountInString(externalID) > maxSubjectFieldLength || utf8.RuneCountInString(relation) > maxSubjectFieldLength:
			detail.NormalizationStatus = subject.NormalizationInvalidSubject
			detail.NormalizationReason = "invalid_subject_fields"
			detail.LocalDate = nil
		case subjectType != string(subject.CustomerType):
			detail.NormalizationStatus = subject.NormalizationUnsupportedType
			detail.NormalizationReason = "unsupported_subject_type"
			detail.LocalDate = nil
		case !operationValid:
			detail.NormalizationStatus = subject.NormalizationInvalidSubject
			detail.NormalizationReason = "invalid_operation"
			detail.LocalDate = nil
		case !lifecycleValid || (valid.EventName == "subject_merged" && mergedIntoID == externalID):
			detail.NormalizationStatus = subject.NormalizationInvalidSubject
			detail.NormalizationReason = "invalid_lifecycle"
			detail.LocalDate = nil
		case !timezoneValid:
			detail.NormalizationStatus = subject.NormalizationTimezoneUnresolved
			detail.NormalizationReason = "timezone_unresolved"
			detail.LocalDate = nil
		}
		details = append(details, detail)
	}
	return details
}

// parseLifecycle extrae la tupla monotónica solo para eventos lifecycle.
func parseLifecycle(eventName string, value any) (*time.Time, string, *time.Time, string, bool) {
	isLifecycle := eventName == "subject_deactivated" || eventName == "subject_reactivated" || eventName == "subject_merged"
	if !isLifecycle {
		return nil, "", nil, "", value == nil
	}
	values, ok := value.(map[string]any)
	if !ok {
		return nil, "", nil, "", false
	}
	updatedAt, ok := parseLifecycleTime(values["source_updated_at"])
	if !ok {
		return nil, "", nil, "", false
	}
	mutationID, _ := values["source_mutation_id"].(string)
	mutationID = strings.TrimSpace(mutationID)
	if mutationID == "" || utf8.RuneCountInString(mutationID) > maxSubjectFieldLength {
		return nil, "", nil, "", false
	}
	var effectiveAt *time.Time
	if rawEffective, exists := values["effective_at"]; exists {
		parsed, valid := parseLifecycleTime(rawEffective)
		if !valid {
			return nil, "", nil, "", false
		}
		effectiveAt = &parsed
	}
	mergedIntoID, _ := values["merged_into_subject_id"].(string)
	mergedIntoID = strings.TrimSpace(mergedIntoID)
	if eventName == "subject_merged" {
		if mergedIntoID == "" || utf8.RuneCountInString(mergedIntoID) > maxSubjectFieldLength {
			return nil, "", nil, "", false
		}
	} else if mergedIntoID != "" {
		return nil, "", nil, "", false
	}
	return &updatedAt, mutationID, effectiveAt, mergedIntoID, true
}

// parseLifecycleTime acepta RFC3339 o time.Time y exige offset UTC.
func parseLifecycleTime(value any) (time.Time, bool) {
	var parsed time.Time
	switch typed := value.(type) {
	case time.Time:
		parsed = typed
	case string:
		value, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(typed))
		if err != nil {
			return time.Time{}, false
		}
		parsed = value
	default:
		return time.Time{}, false
	}
	_, offset := parsed.Zone()
	return parsed, !parsed.IsZero() && offset == 0
}

// subjectList normaliza las representaciones de slices usadas en JSON y tests.
func subjectList(value any) ([]any, bool) {
	switch values := value.(type) {
	case []any:
		return values, true
	case []map[string]any:
		result := make([]any, 0, len(values))
		for _, item := range values {
			result = append(result, item)
		}
		return result, true
	default:
		return nil, false
	}
}

// invalidSubjectDetail crea una fila semántica sin identidad canónica.
func invalidSubjectDetail(ordinal int, reason string) subject.EventDetail {
	return subject.EventDetail{
		SubjectOrdinal:          ordinal,
		ContextResolutionStatus: subject.ContextRegistryDisabled,
		NormalizationStatus:     subject.NormalizationInvalidSubject,
		NormalizationReason:     reason,
		Metadata:                map[string]any{},
		InitialLifecycleStatus:  subject.LifecycleUnknown,
	}
}

// parseOperation extrae una operación opcional con campos completos.
func parseOperation(value any) (string, string, bool) {
	if value == nil {
		return "", "", true
	}
	operation, ok := value.(map[string]any)
	if !ok {
		return "", "", false
	}
	operationType, _ := operation["operation_type"].(string)
	operationID, _ := operation["operation_id"].(string)
	operationType = strings.TrimSpace(operationType)
	operationID = strings.TrimSpace(operationID)
	valid := operationType != "" && operationID != "" && utf8.RuneCountInString(operationType) <= maxSubjectFieldLength && utf8.RuneCountInString(operationID) <= maxSubjectFieldLength
	return operationType, operationID, valid
}

// parseSubjectContext conserva la declaración contextual aún sin registro.
func parseSubjectContext(values map[string]any) (string, *int, string) {
	contextKey, _ := values["context_key"].(string)
	claimedFamily, _ := values["claimed_context_family"].(string)
	version, ok := positiveInteger(values["context_version"])
	if !ok {
		return strings.TrimSpace(contextKey), nil, strings.TrimSpace(claimedFamily)
	}
	return strings.TrimSpace(contextKey), &version, strings.TrimSpace(claimedFamily)
}

// positiveInteger convierte números JSON enteros positivos.
func positiveInteger(value any) (int, bool) {
	var numeric float64
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed, true
		}
		return 0, false
	case float64:
		numeric = typed
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		numeric = parsed
	default:
		return 0, false
	}
	if numeric <= 0 || numeric != math.Trunc(numeric) || numeric > float64(math.MaxInt) {
		return 0, false
	}
	return int(numeric), true
}

// resolveLocalDate calcula la fecha civil usando el snapshot IANA del site.
func resolveLocalDate(eventTime time.Time, timezone string) (*time.Time, bool) {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return nil, false
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, false
	}
	local := eventTime.In(location)
	date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return &date, true
}

// initialSubjectStatus define el alta lazy sin aplicar eventos lifecycle.
func initialSubjectStatus(valid event.ValidatedEvent) subject.LifecycleStatus {
	if valid.EventSource == event.SourceServer && valid.EventName == "subject_associated" {
		return subject.LifecycleActive
	}
	return subject.LifecycleUnknown
}
