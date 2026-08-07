package bootstrap

import "testing"

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
