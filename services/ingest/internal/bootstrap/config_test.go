package bootstrap

import "testing"

func TestLoadConfigSubjectInternalEventsDisabledByDefault(t *testing.T) {
	t.Setenv("SUBJECT_INTERNAL_EVENTS_ENABLED", "")

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.SubjectInternalEventsEnabled {
		t.Fatal("SUBJECT_INTERNAL_EVENTS_ENABLED deberia estar apagado por defecto")
	}
}

func TestLoadConfigRejectsInvalidSubjectInternalEventsFlag(t *testing.T) {
	t.Setenv("SUBJECT_INTERNAL_EVENTS_ENABLED", "tal-vez")

	if _, err := LoadConfig(""); err == nil {
		t.Fatal("LoadConfig() deberia rechazar SUBJECT_INTERNAL_EVENTS_ENABLED invalido")
	}
}

func TestLoadConfigRequiresDedicatedTokenWhenInternalEventsEnabled(t *testing.T) {
	t.Setenv("SUBJECT_INTERNAL_EVENTS_ENABLED", "true")
	t.Setenv("SUBJECT_EVENTS_API_TOKEN", "")

	if _, err := LoadConfig(""); err == nil {
		t.Fatal("LoadConfig() debería exigir SUBJECT_EVENTS_API_TOKEN")
	}
}

func TestLoadConfigAcceptsDedicatedInternalEventsToken(t *testing.T) {
	t.Setenv("SUBJECT_INTERNAL_EVENTS_ENABLED", "true")
	t.Setenv("SUBJECT_EVENTS_API_TOKEN", "internal-secret")

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !config.SubjectInternalEventsEnabled || config.SubjectEventsAPIToken != "internal-secret" {
		t.Fatalf("config interna = %#v", config)
	}
}
