package httpsnapshot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/usecases"
)

func TestClientFetchesAuthenticatedPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" || request.URL.Query().Get("cursor") != "c1" || request.URL.Query().Get("source_revision") != "r1" {
			t.Fatalf("request inesperado: %s", request.URL.String())
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"subject_type":"customer","source_revision":"r1","next_cursor":null,"has_more":false,"subjects":[{"tenant_id":"tenant-a","external_subject_id":"1","lifecycle_status":"active","source_updated_at":"2026-08-10T12:00:00Z","source_mutation_id":"m1","merged_into_external_subject_id":null}]}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.FetchPage(context.Background(), "c1", "r1", 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].TenantID != "tenant-a" {
		t.Fatalf("page=%#v", page)
	}
}

func TestClientMapsExpiredRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusConflict) }))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret", time.Second)
	if _, err := client.FetchPage(context.Background(), "c1", "r1", 10); err != usecases.ErrSnapshotRevisionExpired {
		t.Fatalf("err=%v", err)
	}
}

func TestClientRetriesTemporaryFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte(`{"subject_type":"customer","source_revision":"r1","next_cursor":null,"has_more":false,"subjects":[]}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret", time.Second)
	if _, err := client.FetchPage(context.Background(), "", "", 10); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d", attempts)
	}
}
