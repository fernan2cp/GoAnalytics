package bootstrap

import (
	"strings"
	"testing"
)

func TestLoadConfigPendingClaimDefaults(t *testing.T) {
	t.Setenv("WORKER_PENDING_CLAIM_ENABLED", "")
	t.Setenv("WORKER_PENDING_MIN_IDLE_MS", "")
	t.Setenv("WORKER_PENDING_CLAIM_INTERVAL_MS", "")
	t.Setenv("WORKER_PENDING_CLAIM_BATCH_SIZE", "")
	t.Setenv("WORKER_MAX_DELIVERIES", "")
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !config.WorkerPendingClaimEnabled || config.WorkerPendingClaimBatchSize != 100 || config.WorkerMaxDeliveries != 5 {
		t.Fatalf("configuracion pending inesperada: %#v", config)
	}
}

func TestLoadConfigRejectsInvalidPendingClaimBoolean(t *testing.T) {
	t.Setenv("WORKER_PENDING_CLAIM_ENABLED", "tal-vez")
	if _, err := LoadConfig(""); err == nil {
		t.Fatalf("LoadConfig() deberia rechazar booleano invalido")
	}
}

func TestLoadConfigSubjectFeaturesDisabledByDefault(t *testing.T) {
	clearSubjectFeatureFlags(t)

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.SubjectAnalyticsEnabled || config.SubjectBootstrapEnabled ||
		config.SubjectProjectionEnabled || config.SubjectContextRegistryEnabled ||
		config.SubjectQueryAPIEnabled {
		t.Fatalf("las capacidades de sujetos deberian iniciar apagadas: %#v", config)
	}
}

func TestLoadConfigRejectsInvalidSubjectFeatureCombinations(t *testing.T) {
	tests := []struct {
		name        string
		environment map[string]string
		wantError   string
	}{
		{name: "bootstrap sin normalizacion", environment: map[string]string{"SUBJECT_BOOTSTRAP_ENABLED": "true"}, wantError: "requiere SUBJECT_ANALYTICS_ENABLED"},
		{name: "proyeccion sin normalizacion", environment: map[string]string{"SUBJECT_PROJECTION_ENABLED": "true"}, wantError: "requiere SUBJECT_ANALYTICS_ENABLED"},
		{name: "registro sin normalizacion", environment: map[string]string{"SUBJECT_CONTEXT_REGISTRY_ENABLED": "true"}, wantError: "requiere SUBJECT_ANALYTICS_ENABLED"},
		{name: "api sin proyeccion", environment: map[string]string{"SUBJECT_ANALYTICS_ENABLED": "true", "SUBJECT_QUERY_API_ENABLED": "true"}, wantError: "requiere SUBJECT_PROJECTION_ENABLED"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearSubjectFeatureFlags(t)
			for key, value := range test.environment {
				t.Setenv(key, value)
			}
			_, err := LoadConfig("")
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("LoadConfig() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestLoadConfigAcceptsCompleteSubjectFeatureChain(t *testing.T) {
	clearSubjectFeatureFlags(t)
	t.Setenv("SUBJECT_ANALYTICS_ENABLED", "true")
	t.Setenv("SUBJECT_BOOTSTRAP_ENABLED", "true")
	t.Setenv("SUBJECT_PROJECTION_ENABLED", "true")
	t.Setenv("SUBJECT_CONTEXT_REGISTRY_ENABLED", "true")
	t.Setenv("SUBJECT_QUERY_API_ENABLED", "true")

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !config.SubjectAnalyticsEnabled || !config.SubjectBootstrapEnabled ||
		!config.SubjectProjectionEnabled || !config.SubjectContextRegistryEnabled ||
		!config.SubjectQueryAPIEnabled {
		t.Fatalf("cadena de capacidades incompleta: %#v", config)
	}
}

func TestLoadConfigRejectsInvalidSubjectFeatureBoolean(t *testing.T) {
	clearSubjectFeatureFlags(t)
	t.Setenv("SUBJECT_ANALYTICS_ENABLED", "tal-vez")

	if _, err := LoadConfig(""); err == nil {
		t.Fatal("LoadConfig() deberia rechazar un flag de sujetos invalido")
	}
}

// clearSubjectFeatureFlags aisla la matriz de configuracion entre pruebas.
func clearSubjectFeatureFlags(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"SUBJECT_ANALYTICS_ENABLED",
		"SUBJECT_BOOTSTRAP_ENABLED",
		"SUBJECT_PROJECTION_ENABLED",
		"SUBJECT_CONTEXT_REGISTRY_ENABLED",
		"SUBJECT_QUERY_API_ENABLED",
	} {
		t.Setenv(key, "")
	}
}
