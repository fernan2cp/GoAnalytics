package postgres

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"goanalytics/services/worker/internal/adapters/inbound/httpsuggestions"
	"goanalytics/services/worker/internal/application/usecases"
)

// TestSubjectMVPVolumeProfile certifica el máximo acordado contra PostgreSQL real.
func TestSubjectMVPVolumeProfile(t *testing.T) {
	if os.Getenv("SUBJECT_MVP_VOLUME_TEST") != "1" {
		t.Skip("SUBJECT_MVP_VOLUME_TEST no habilitado")
	}
	dsn := os.Getenv("POSTGRES_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_INTEGRATION_DSN no configurada")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("task_ca_0408_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, mvpVolumeSchema); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO analytics_subjects(id,tenant_id,external_subject_id) SELECT value,'tenant-'||lpad(((value-1)%20)::text,2,'0'),'customer-'||lpad(value::text,6,'0') FROM generate_series(1,100000) value; INSERT INTO analytics_event_subjects(tenant_id,analytics_subject_id,context_definition_id,local_date,timezone_name,event_name,event_time,updated_at,normalization_status,context_resolution_status) SELECT 'tenant-'||lpad(((value-1)%20)::text,2,'0'),((value-1)%100000)+1,1,current_date,'UTC','subject_associated',now(),TIMESTAMPTZ '2026-08-11 12:00:00+00','normalized','registered' FROM generate_series(1,500000) value`); err != nil {
		t.Fatal(err)
	}
	repository, _ := NewProjectionRepository(pool)
	global, _ := usecases.NewProjectGlobal(repository, nil, 1, 500000)
	contextual, _ := usecases.NewProjectContext(repository, 1, 500000)
	projectionStarted := time.Now()
	if err = global.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err = contextual.Run(ctx); err != nil {
		t.Fatal(err)
	}
	projectionDuration := time.Since(projectionStarted)
	if projectionDuration > 30*time.Minute {
		t.Fatalf("proyección=%s", projectionDuration)
	}
	for _, table := range []string{"analytics_subject_tenant_daily", "analytics_subject_context_daily"} {
		var rows, total, tenants int64
		if err = pool.QueryRow(ctx, "SELECT count(*),sum(association_count),count(DISTINCT tenant_id) FROM "+table).Scan(&rows, &total, &tenants); err != nil {
			t.Fatal(err)
		}
		if rows != 100000 || total != 500000 || tenants != 20 {
			t.Fatalf("%s rows=%d total=%d tenants=%d", table, rows, total, tenants)
		}
	}
	suggestions, _ := NewSuggestionRepository(pool)
	query := usecases.NewQuerySuggestions(suggestions, 1, 90, 30, 48*time.Hour, true)
	server := httptest.NewServer(httpsuggestions.NewHandler(true, "volume-token", 10, 100, query))
	defer server.Close()
	client := server.Client()
	durations := make([]time.Duration, 0, 200)
	for index := 0; index < 100; index++ {
		for _, suffix := range []string{"", "&context_key=sales.customer_association&context_version=1"} {
			started := time.Now()
			request, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/internal/subjects/suggestions?tenant_id=tenant-00&subject_type=customer&limit=10"+suffix, nil)
			request.Header.Set("Authorization", "Bearer volume-token")
			response, requestErr := client.Do(request)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			_, readErr := io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if readErr != nil || response.StatusCode != http.StatusOK {
				t.Fatalf("status=%d err=%v", response.StatusCode, readErr)
			}
			durations = append(durations, time.Since(started))
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[(len(durations)*95/100)-1]
	if p95 > 300*time.Millisecond {
		t.Fatalf("query p95=%s", p95)
	}
	t.Logf("profile tenants=20 subjects=100000 events=500000 projection=%s query_http_p95=%s", projectionDuration, p95)
}

const mvpVolumeSchema = `
CREATE TABLE analytics_subjects(id BIGINT PRIMARY KEY,tenant_id text,external_subject_id text,lifecycle_status text DEFAULT 'active');
CREATE TABLE analytics_event_subjects(id BIGSERIAL PRIMARY KEY,tenant_id text,analytics_subject_id bigint,context_definition_id bigint,local_date date,timezone_name text,event_name text,event_time timestamptz,updated_at timestamptz,normalization_status text,context_resolution_status text);
CREATE INDEX ix_volume_watermark ON analytics_event_subjects(updated_at,id);CREATE INDEX ix_volume_day ON analytics_event_subjects(tenant_id,analytics_subject_id,local_date,timezone_name);
CREATE TABLE analytics_projection_runs(id BIGSERIAL PRIMARY KEY,projection_name text,projection_version int,status text,started_at timestamptz,completed_at timestamptz,source_from_updated_at timestamptz,source_from_id bigint,source_to_updated_at timestamptz,source_to_id bigint,affected_days int DEFAULT 0,rows_read bigint DEFAULT 0,rows_written bigint DEFAULT 0,last_error text);
CREATE TABLE analytics_projection_checkpoints(projection_name text,projection_version int,last_updated_at timestamptz,last_id bigint,updated_at timestamptz,PRIMARY KEY(projection_name,projection_version));
CREATE TABLE analytics_subject_tenant_daily(tenant_id text,analytics_subject_id bigint,local_date date,timezone_name text,impression_count bigint,view_count bigint,selection_count bigint,association_count bigint,last_association_at timestamptz,projection_version int,computed_at timestamptz,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),PRIMARY KEY(tenant_id,analytics_subject_id,local_date,timezone_name,projection_version));CREATE INDEX ix_volume_global ON analytics_subject_tenant_daily(tenant_id,projection_version,local_date,analytics_subject_id);
CREATE TABLE analytics_context_registry_revisions(id bigint PRIMARY KEY,validation_status text,activated_at timestamptz);INSERT INTO analytics_context_registry_revisions VALUES(1,'valid',now());
CREATE TABLE analytics_context_definitions(id bigint PRIMARY KEY,context_key text,context_version int);INSERT INTO analytics_context_definitions VALUES(1,'sales.customer_association',1);
CREATE TABLE analytics_context_registry_entries(registry_revision_id bigint,context_definition_id bigint,status text);INSERT INTO analytics_context_registry_entries VALUES(1,1,'active');
CREATE TABLE analytics_subject_context_daily(tenant_id text,analytics_subject_id bigint,context_definition_id bigint,local_date date,timezone_name text,impression_count bigint,view_count bigint,selection_count bigint,association_count bigint,last_association_at timestamptz,projection_version int,computed_at timestamptz,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),PRIMARY KEY(tenant_id,analytics_subject_id,context_definition_id,local_date,timezone_name,projection_version));CREATE INDEX ix_volume_context ON analytics_subject_context_daily(tenant_id,context_definition_id,projection_version,local_date,analytics_subject_id);`
