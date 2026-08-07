package redisstream

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/domain/rejection"
)

func TestProcessMessagesIsolatesInvalidPayload(t *testing.T) {
	client := newFakeStreamClient()
	processor := &fakeEventProcessor{}
	consumer := newTestConsumer(t, client, processor)

	consumer.processMessages(context.Background(), []goredis.XMessage{
		{ID: "1-0", Values: map[string]any{"payload": "{"}},
		{ID: "2-0", Values: map[string]any{"payload": "{}"}},
	}, nil)

	if len(processor.events) != 1 {
		t.Fatalf("eventos procesados = %d, want 1", len(processor.events))
	}
	if len(processor.rejections) != 1 || processor.rejections[0].Reason != rejection.ReasonInvalidStreamPayload {
		t.Fatalf("rechazos = %#v, want invalid_stream_payload", processor.rejections)
	}
	assertAckedIDs(t, client.acked, "1-0", "2-0")
}

func TestProcessMessagesKeepsRejectedPendingWhenPersistenceFails(t *testing.T) {
	client := newFakeStreamClient()
	processor := &fakeEventProcessor{rejectionErr: errors.New("postgres no disponible")}
	consumer := newTestConsumer(t, client, processor)

	consumer.processMessages(context.Background(), []goredis.XMessage{
		{ID: "1-0", Values: map[string]any{"payload": "{"}},
		{ID: "2-0", Values: map[string]any{"payload": "{}"}},
	}, nil)

	assertAckedIDs(t, client.acked, "2-0")
}

func TestProcessMessagePersistsRetryExhaustedBeforeAck(t *testing.T) {
	client := newFakeStreamClient()
	processor := &fakeEventProcessor{}
	consumer := newTestConsumer(t, client, processor)
	message := goredis.XMessage{ID: "5-0", Values: map[string]any{"payload": `{"event_id":"evt-5"}`}}

	consumer.processMessage(context.Background(), message, 5)

	if len(processor.events) != 0 {
		t.Fatalf("eventos procesados = %d, want 0", len(processor.events))
	}
	if len(processor.rejections) != 1 {
		t.Fatalf("rechazos = %d, want 1", len(processor.rejections))
	}
	got := processor.rejections[0]
	if got.Reason != rejection.ReasonStreamRetryExhausted || got.DeliveryCount != 5 || got.EventID != "evt-5" {
		t.Fatalf("rechazo = %#v", got)
	}
	assertAckedIDs(t, client.acked, "5-0")
}

func TestClaimPendingUsesAutoClaimAndDeliveryCount(t *testing.T) {
	client := newFakeStreamClient()
	client.claims = []fakeClaimResult{{
		messages: []goredis.XMessage{{ID: "3-0", Values: map[string]any{"payload": "{}"}}},
		next:     "0-0",
	}}
	client.pending["3-0"] = 2
	processor := &fakeEventProcessor{}
	consumer := newTestConsumer(t, client, processor)

	if err := consumer.claimPending(context.Background()); err != nil {
		t.Fatalf("claimPending() error = %v", err)
	}
	if len(client.claimArgs) != 1 {
		t.Fatalf("XAUTOCLAIM llamadas = %d, want 1", len(client.claimArgs))
	}
	args := client.claimArgs[0]
	if args.Start != "0-0" || args.MinIdle != time.Minute || args.Count != 100 {
		t.Fatalf("XAUTOCLAIM args = %#v", args)
	}
	if len(processor.events) != 1 {
		t.Fatalf("eventos procesados = %d, want 1", len(processor.events))
	}
	assertAckedIDs(t, client.acked, "3-0")
}

func TestClaimPendingDoesNotProcessActiveMessage(t *testing.T) {
	client := newFakeStreamClient()
	client.claims = []fakeClaimResult{{next: "0-0"}}
	processor := &fakeEventProcessor{}
	consumer := newTestConsumer(t, client, processor)

	if err := consumer.claimPending(context.Background()); err != nil {
		t.Fatalf("claimPending() error = %v", err)
	}
	if len(processor.events) != 0 || len(processor.rejections) != 0 || len(client.acked) != 0 {
		t.Fatalf("mensaje activo fue procesado")
	}
}

func TestClaimPendingContinuesWhenScanPageHasNoEligibleMessages(t *testing.T) {
	client := newFakeStreamClient()
	client.claims = []fakeClaimResult{
		{next: "9-0"},
		{messages: []goredis.XMessage{{ID: "10-0", Values: map[string]any{"payload": "{}"}}}, next: "0-0"},
	}
	client.pending["10-0"] = 2
	processor := &fakeEventProcessor{}
	consumer := newTestConsumer(t, client, processor)

	if err := consumer.claimPending(context.Background()); err != nil {
		t.Fatalf("claimPending() error = %v", err)
	}
	if len(client.claimArgs) != 2 || client.claimArgs[1].Start != "9-0" {
		t.Fatalf("cursores XAUTOCLAIM = %#v", client.claimArgs)
	}
	assertAckedIDs(t, client.acked, "10-0")
}

func TestShouldClaimPendingHonorsInterval(t *testing.T) {
	client := newFakeStreamClient()
	consumer := newTestConsumer(t, client, &fakeEventProcessor{})
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	consumer.now = func() time.Time { return now }

	if !consumer.shouldClaimPending() {
		t.Fatalf("primera recuperacion deberia ejecutarse")
	}
	consumer.lastPendingClaim = now
	if consumer.shouldClaimPending() {
		t.Fatalf("recuperacion anticipada no deberia ejecutarse")
	}
	now = now.Add(30 * time.Second)
	if !consumer.shouldClaimPending() {
		t.Fatalf("recuperacion vencida deberia ejecutarse")
	}
}

func newTestConsumer(t *testing.T, client streamClient, processor eventProcessor) *Consumer {
	t.Helper()
	consumer, err := NewConsumer(client, Config{
		StreamName:            "events",
		GroupName:             "workers",
		ConsumerName:          "worker-1",
		BatchSize:             10,
		PollInterval:          time.Millisecond,
		PendingClaimEnabled:   true,
		PendingMinIdle:        time.Minute,
		PendingClaimInterval:  30 * time.Second,
		PendingClaimBatchSize: 100,
		MaxDeliveries:         5,
	}, processor, nil)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	return consumer
}

func assertAckedIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("acks = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("acks = %#v, want %#v", got, want)
		}
	}
}

type fakeEventProcessor struct {
	events       []dto.RawEvent
	rejections   []dto.StreamRejection
	processErr   error
	rejectionErr error
}

func (fake *fakeEventProcessor) Process(_ context.Context, events []dto.RawEvent) error {
	fake.events = append(fake.events, events...)
	return fake.processErr
}

func (fake *fakeEventProcessor) RecordStreamRejection(_ context.Context, input dto.StreamRejection) error {
	fake.rejections = append(fake.rejections, input)
	return fake.rejectionErr
}

type fakeClaimResult struct {
	messages []goredis.XMessage
	next     string
	err      error
}

type fakeStreamClient struct {
	claims    []fakeClaimResult
	claimArgs []goredis.XAutoClaimArgs
	pending   map[string]int64
	read      []goredis.XMessage
	acked     []string
	ackErr    error
}

func newFakeStreamClient() *fakeStreamClient {
	return &fakeStreamClient{pending: make(map[string]int64)}
}

func (fake *fakeStreamClient) XGroupCreateMkStream(ctx context.Context, _, _, _ string) *goredis.StatusCmd {
	cmd := goredis.NewStatusCmd(ctx)
	cmd.SetVal("OK")
	return cmd
}

func (fake *fakeStreamClient) XReadGroup(ctx context.Context, _ *goredis.XReadGroupArgs) *goredis.XStreamSliceCmd {
	cmd := goredis.NewXStreamSliceCmd(ctx)
	if len(fake.read) > 0 {
		cmd.SetVal([]goredis.XStream{{Stream: "events", Messages: fake.read}})
	} else {
		cmd.SetVal(nil)
	}
	return cmd
}

func (fake *fakeStreamClient) XAutoClaim(ctx context.Context, args *goredis.XAutoClaimArgs) *goredis.XAutoClaimCmd {
	fake.claimArgs = append(fake.claimArgs, *args)
	cmd := goredis.NewXAutoClaimCmd(ctx)
	if len(fake.claims) == 0 {
		cmd.SetVal(nil, "0-0")
		return cmd
	}
	result := fake.claims[0]
	fake.claims = fake.claims[1:]
	if result.err != nil {
		cmd.SetErr(result.err)
		return cmd
	}
	cmd.SetVal(result.messages, result.next)
	return cmd
}

func (fake *fakeStreamClient) XPendingExt(ctx context.Context, args *goredis.XPendingExtArgs) *goredis.XPendingExtCmd {
	cmd := goredis.NewXPendingExtCmd(ctx)
	count, ok := fake.pending[args.Start]
	if !ok {
		cmd.SetVal(nil)
		return cmd
	}
	cmd.SetVal([]goredis.XPendingExt{{ID: args.Start, Consumer: "worker-1", RetryCount: count}})
	return cmd
}

func (fake *fakeStreamClient) XAck(ctx context.Context, _, _ string, ids ...string) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(ctx)
	if fake.ackErr != nil {
		cmd.SetErr(fake.ackErr)
		return cmd
	}
	fake.acked = append(fake.acked, ids...)
	cmd.SetVal(int64(len(ids)))
	return cmd
}
