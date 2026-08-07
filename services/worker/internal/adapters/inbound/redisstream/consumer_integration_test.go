package redisstream

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestClaimPendingIntegrationRecoversAbandonedMessage(t *testing.T) {
	address := os.Getenv("REDIS_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("REDIS_INTEGRATION_ADDR no configurada")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	stream := fmt.Sprintf("test:task-ca-0001:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(ctx, stream).Err() })

	if err := client.XGroupCreateMkStream(ctx, stream, "workers", "0").Err(); err != nil {
		t.Fatalf("crear grupo: %v", err)
	}
	messageID, err := client.XAdd(ctx, &goredis.XAddArgs{Stream: stream, Values: map[string]any{"payload": "{}"}}).Result()
	if err != nil {
		t.Fatalf("publicar mensaje: %v", err)
	}
	if _, err := client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group: "workers", Consumer: "worker-caido", Streams: []string{stream, ">"}, Count: 1,
	}).Result(); err != nil {
		t.Fatalf("leer con consumer caido: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	processor := &fakeEventProcessor{}
	consumer, err := NewConsumer(client, Config{
		StreamName: stream, GroupName: "workers", ConsumerName: "worker-recuperador",
		PendingClaimEnabled: true, PendingMinIdle: 10 * time.Millisecond,
		PendingClaimInterval: time.Second, PendingClaimBatchSize: 10, MaxDeliveries: 5,
	}, processor, nil)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	if err := consumer.claimPending(ctx); err != nil {
		t.Fatalf("claimPending() error = %v", err)
	}
	if len(processor.events) != 1 {
		t.Fatalf("eventos procesados = %d, want 1", len(processor.events))
	}
	pending, err := client.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: stream, Group: "workers", Start: messageID, End: messageID, Count: 1,
	}).Result()
	if err != nil {
		t.Fatalf("consultar pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("mensaje recuperado continua pendiente: %#v", pending)
	}
}

func TestClaimPendingIntegrationDoesNotClaimActiveMessage(t *testing.T) {
	address := os.Getenv("REDIS_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("REDIS_INTEGRATION_ADDR no configurada")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	stream := fmt.Sprintf("test:task-ca-0001-active:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(ctx, stream).Err() })

	if err := client.XGroupCreateMkStream(ctx, stream, "workers", "0").Err(); err != nil {
		t.Fatalf("crear grupo: %v", err)
	}
	if _, err := client.XAdd(ctx, &goredis.XAddArgs{Stream: stream, Values: map[string]any{"payload": "{}"}}).Result(); err != nil {
		t.Fatalf("publicar mensaje: %v", err)
	}
	if _, err := client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group: "workers", Consumer: "worker-activo", Streams: []string{stream, ">"}, Count: 1,
	}).Result(); err != nil {
		t.Fatalf("leer con consumer activo: %v", err)
	}

	processor := &fakeEventProcessor{}
	consumer, err := NewConsumer(client, Config{
		StreamName: stream, GroupName: "workers", ConsumerName: "worker-recuperador",
		PendingClaimEnabled: true, PendingMinIdle: time.Hour,
		PendingClaimInterval: time.Second, PendingClaimBatchSize: 10, MaxDeliveries: 5,
	}, processor, nil)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	if err := consumer.claimPending(ctx); err != nil {
		t.Fatalf("claimPending() error = %v", err)
	}
	if len(processor.events) != 0 {
		t.Fatalf("mensaje activo fue reclamado")
	}
}
