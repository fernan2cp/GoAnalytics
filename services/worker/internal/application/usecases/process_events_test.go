package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/event"
	"goanalytics/services/worker/internal/domain/rejection"
	"goanalytics/services/worker/internal/domain/site"
)

func TestProcessValidEventPersistsAndMarksDeduplicator(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	eventRepository := &fakeEventRepository{}
	deduplicator := &fakeDeduplicator{}
	useCase := newTestProcessUseCase(now, eventRepository, &fakeRejectedRepository{}, cachedSite(), nil, deduplicator)

	if err := useCase.Process(context.Background(), []dto.RawEvent{validRawEvent()}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
	persisted := eventRepository.events[0]
	if persisted.TenantID != "tenant_123" || persisted.SiteID != "site_456" {
		t.Fatalf("metadata real = %q/%q, want tenant_123/site_456", persisted.TenantID, persisted.SiteID)
	}
	if persisted.Properties == nil || persisted.Context == nil {
		t.Fatalf("Properties y Context deben persistirse como mapas no nulos")
	}
	if len(deduplicator.marked) != 1 || deduplicator.marked[0].Key != "evt_1" {
		t.Fatalf("marked = %#v, want evt_1", deduplicator.marked)
	}
}

func TestProcessCacheMissRehydratesAndCachesSite(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	cache := &fakeSiteCache{found: false}
	resolver := &fakeSiteResolver{config: cachedSite()}
	eventRepository := &fakeEventRepository{}
	useCase := newProcessUseCaseWithPorts(now, eventRepository, &fakeRejectedRepository{}, cache, resolver, &fakeDeduplicator{})

	if err := useCase.Process(context.Background(), []dto.RawEvent{validRawEvent()}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolver.calls)
	}
	if len(cache.sets) != 1 {
		t.Fatalf("cache sets = %d, want 1", len(cache.sets))
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
}

func TestProcessRejectsDomainNotAllowedAsSuspicious(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	rejectedRepository := &fakeRejectedRepository{}
	useCase := newTestProcessUseCase(now, &fakeEventRepository{}, rejectedRepository, cachedSite(), nil, &fakeDeduplicator{})
	raw := validRawEvent()
	raw.Origin = "https://intruso.example"

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(rejectedRepository.events) != 1 {
		t.Fatalf("rejected events = %d, want 1", len(rejectedRepository.events))
	}
	rejected := rejectedRepository.events[0]
	if rejected.Reason != rejection.ReasonDomainNotAllowed {
		t.Fatalf("Reason = %q, want %q", rejected.Reason, rejection.ReasonDomainNotAllowed)
	}
	if rejected.Severity != rejection.SeveritySuspicious {
		t.Fatalf("Severity = %q, want %q", rejected.Severity, rejection.SeveritySuspicious)
	}
}

func TestProcessRejectsInactiveSiteAndTokenMismatch(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		config     site.SiteConfig
		raw        dto.RawEvent
		wantReason string
	}{
		{
			name: "inactive",
			config: func() site.SiteConfig {
				config := cachedSite()
				config.Status = "disabled"
				return config
			}(),
			raw:        validRawEvent(),
			wantReason: rejection.ReasonSiteInactive,
		},
		{
			name: "token mismatch",
			config: func() site.SiteConfig {
				config := cachedSite()
				config.TokenVersion = 2
				return config
			}(),
			raw:        validRawEvent(),
			wantReason: rejection.ReasonTokenVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rejectedRepository := &fakeRejectedRepository{}
			useCase := newTestProcessUseCase(now, &fakeEventRepository{}, rejectedRepository, tt.config, nil, &fakeDeduplicator{})

			if err := useCase.Process(context.Background(), []dto.RawEvent{tt.raw}); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if len(rejectedRepository.events) != 1 {
				t.Fatalf("rejected events = %d, want 1", len(rejectedRepository.events))
			}
			if rejectedRepository.events[0].Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", rejectedRepository.events[0].Reason, tt.wantReason)
			}
		})
	}
}

func TestProcessRejectsDuplicateWithoutPersistingValidEvent(t *testing.T) {
	rejectedRepository := &fakeRejectedRepository{}
	eventRepository := &fakeEventRepository{}
	useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, &fakeDeduplicator{seen: true})

	if err := useCase.Process(context.Background(), []dto.RawEvent{validRawEvent()}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 0 {
		t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
	}
	if len(rejectedRepository.events) != 1 || rejectedRepository.events[0].Reason != rejection.ReasonDuplicateEvent {
		t.Fatalf("rejected = %#v, want duplicate_event", rejectedRepository.events)
	}
}

func TestProcessRejectsDuplicateLogicalEventWithoutSemanticFallback(t *testing.T) {
	rejectedRepository := &fakeRejectedRepository{}
	eventRepository := &fakeEventRepository{}
	raw := validRawEvent()
	raw.LogicalEventID = "logical_page_1"
	deduplicator := &fakeDeduplicator{seenByStrategy: map[string]bool{dedupStrategyLogical: true}}
	useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, deduplicator)

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 0 {
		t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
	}
	if len(rejectedRepository.events) != 1 || rejectedRepository.events[0].Reason != rejection.ReasonDuplicateLogicalEvent {
		t.Fatalf("rejected = %#v, want duplicate_logical_event", rejectedRepository.events)
	}
	if rejectedRepository.events[0].RawPayload["dedup_strategy"] != dedupStrategyLogical {
		t.Fatalf("dedup_strategy = %#v, want logical", rejectedRepository.events[0].RawPayload["dedup_strategy"])
	}
}

func TestProcessAppliesSemanticDedupOnlyWhenNoStrongKey(t *testing.T) {
	rejectedRepository := &fakeRejectedRepository{}
	eventRepository := &fakeEventRepository{}
	raw := validRawEvent()
	deduplicator := &fakeDeduplicator{seenByStrategy: map[string]bool{dedupStrategySemantic: true}}
	useCase := newProcessUseCaseWithPorts(time.Now(), eventRepository, rejectedRepository, &fakeSiteCache{config: cachedSite(), found: true}, &fakeSiteResolver{config: cachedSite()}, deduplicator)
	useCase.semanticRules = []SemanticDedupRule{{EventName: "page_view", Window: 200 * time.Millisecond, Fields: []string{"session_id", "tab_id", "path"}}}

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 0 {
		t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
	}
	if len(rejectedRepository.events) != 1 || rejectedRepository.events[0].Reason != rejection.ReasonDuplicateSemanticEvent {
		t.Fatalf("rejected = %#v, want duplicate_semantic_event", rejectedRepository.events)
	}
}

func TestProcessReturnsPersistenceErrors(t *testing.T) {
	useCase := newTestProcessUseCase(
		time.Now(),
		&fakeEventRepository{err: errors.New("db caida")},
		&fakeRejectedRepository{},
		cachedSite(),
		nil,
		&fakeDeduplicator{},
	)

	err := useCase.Process(context.Background(), []dto.RawEvent{validRawEvent()})
	if !errors.Is(err, ErrPersistValidEventsFailed) {
		t.Fatalf("Process() error = %v, want ErrPersistValidEventsFailed", err)
	}
}

func newTestProcessUseCase(
	now time.Time,
	eventRepository *fakeEventRepository,
	rejectedRepository *fakeRejectedRepository,
	config site.SiteConfig,
	resolverErr error,
	deduplicator *fakeDeduplicator,
) *ProcessEventsUseCase {
	cache := &fakeSiteCache{config: config, found: true}
	resolver := &fakeSiteResolver{config: config, err: resolverErr}
	return newProcessUseCaseWithPorts(now, eventRepository, rejectedRepository, cache, resolver, deduplicator)
}

func newProcessUseCaseWithPorts(
	now time.Time,
	eventRepository *fakeEventRepository,
	rejectedRepository *fakeRejectedRepository,
	cache *fakeSiteCache,
	resolver *fakeSiteResolver,
	deduplicator *fakeDeduplicator,
) *ProcessEventsUseCase {
	return NewProcessEventsUseCase(
		nil,
		eventRepository,
		rejectedRepository,
		cache,
		resolver,
		deduplicator,
		fakeClock{now: now},
		nil,
	)
}

func validRawEvent() dto.RawEvent {
	eventTime := time.Date(2026, 5, 5, 11, 59, 0, 0, time.UTC)
	receivedAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	return dto.RawEvent{
		EventID:      "evt_1",
		SiteCode:     "pub_site_abc123",
		Environment:  "production",
		TokenVersion: 1,
		JWTID:        "jti_123",
		EventName:    "page_view",
		EventVersion: 1,
		EventTime:    eventTime,
		ReceivedAt:   receivedAt,
		AnonymousID:  "anon_abc",
		SessionID:    "sess_xyz",
		Origin:       "https://cliente.com",
		URL:          "https://cliente.com/productos/123",
		Path:         "/productos/123",
		UserAgent:    "Mozilla/5.0",
		IPHash:       "ip_hash_123",
		SDKName:      "goanalytics-web",
		SDKVersion:   "1.0.0",
	}
}

func cachedSite() site.SiteConfig {
	return site.SiteConfig{
		SiteCode:        "pub_site_abc123",
		TenantID:        "tenant_123",
		SiteID:          "site_456",
		Status:          "active",
		TrackingEnabled: true,
		AllowedDomains:  []string{"cliente.com", "www.cliente.com"},
		TokenVersion:    1,
		SampleRate:      1,
		SchemaVersion:   1,
	}
}

type fakeEventRepository struct {
	events []event.ValidatedEvent
	err    error
}

func (fake *fakeEventRepository) SaveBatch(ctx context.Context, events []event.ValidatedEvent) error {
	_ = ctx
	fake.events = append(fake.events, events...)
	return fake.err
}

type fakeRejectedRepository struct {
	events []rejection.RejectedEvent
	err    error
}

func (fake *fakeRejectedRepository) SaveBatch(ctx context.Context, events []rejection.RejectedEvent) error {
	_ = ctx
	fake.events = append(fake.events, events...)
	return fake.err
}

type fakeSiteCache struct {
	config site.SiteConfig
	found  bool
	err    error
	sets   []site.SiteConfig
	keys   []outbound.SiteCacheKey
}

func (fake *fakeSiteCache) Get(ctx context.Context, key outbound.SiteCacheKey) (site.SiteConfig, bool, error) {
	_ = ctx
	fake.keys = append(fake.keys, key)
	return fake.config, fake.found, fake.err
}

func (fake *fakeSiteCache) Set(ctx context.Context, key outbound.SiteCacheKey, config site.SiteConfig, ttl time.Duration) error {
	_ = ctx
	fake.keys = append(fake.keys, key)
	_ = ttl
	fake.sets = append(fake.sets, config)
	return fake.err
}

type fakeSiteResolver struct {
	config site.SiteConfig
	err    error
	calls  int
}

func (fake *fakeSiteResolver) Resolve(ctx context.Context, input outbound.ResolveSiteInput) (site.SiteConfig, error) {
	_ = ctx
	_ = input
	fake.calls++
	return fake.config, fake.err
}

type fakeDeduplicator struct {
	seen           bool
	seenByStrategy map[string]bool
	err            error
	marked         []outbound.DeduplicationKey
}

func (fake *fakeDeduplicator) Seen(ctx context.Context, key outbound.DeduplicationKey) (bool, error) {
	_ = ctx
	if fake.seenByStrategy != nil {
		return fake.seenByStrategy[key.Strategy], fake.err
	}
	return fake.seen, fake.err
}

func (fake *fakeDeduplicator) Mark(ctx context.Context, key outbound.DeduplicationKey) error {
	_ = ctx
	fake.marked = append(fake.marked, key)
	return fake.err
}

type fakeClock struct {
	now time.Time
}

func (fake fakeClock) Now() time.Time {
	return fake.now
}
