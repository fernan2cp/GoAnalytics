package redis

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/ingest/internal/domain/event"
)

func TestEventStreamPublisherIntegrationPreservesSource(t *testing.T) {
	address := os.Getenv("REDIS_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("REDIS_INTEGRATION_ADDR no configurada")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	stream := "task-ca-0103:" + time.Now().UTC().Format("20060102150405.000000000")
	t.Cleanup(func() { _ = client.Del(ctx, stream).Err() })

	publisher, err := NewEventStreamPublisher(client, stream, 0)
	if err != nil {
		t.Fatalf("NewEventStreamPublisher() error = %v", err)
	}
	input := event.RawEvent{EventID: "server-1", EventSource: event.SourceServer, SiteCode: "site-1"}
	if err := publisher.Publish(ctx, []event.RawEvent{input}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	messages, err := client.XRange(ctx, stream, "-", "+").Result()
	if err != nil || len(messages) != 1 {
		t.Fatalf("XRange() mensajes=%d error=%v", len(messages), err)
	}
	if messages[0].Values["event_source"] != event.SourceServer {
		t.Fatalf("source del envelope = %#v", messages[0].Values["event_source"])
	}
	var payload event.RawEvent
	if err := json.Unmarshal([]byte(messages[0].Values["payload"].(string)), &payload); err != nil {
		t.Fatalf("payload JSON inválido: %v", err)
	}
	if payload.EventSource != event.SourceServer {
		t.Fatalf("source del payload = %q", payload.EventSource)
	}
}
