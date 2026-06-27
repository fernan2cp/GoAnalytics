package usecases

import (
	"context"
	"errors"
	"strings"
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

func TestProcessAcceptsSafeUnknownContextFields(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	eventRepository := &fakeEventRepository{}
	useCase := newTestProcessUseCase(now, eventRepository, &fakeRejectedRepository{}, cachedSite(), nil, &fakeDeduplicator{})
	raw := validRawEvent()
	raw.Context = map[string]any{
		"app_area": "backoffice",
		"feature":  "catalog_search",
		"surface":  "drawer",
		"runtime": map[string]any{
			"safe_unknown": "accepted",
		},
	}

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
	if eventRepository.events[0].Context["feature"] != "catalog_search" {
		t.Fatalf("context = %#v, want feature preserved", eventRepository.events[0].Context)
	}
}

func TestProcessRejectsSensitivePayloadKeys(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*dto.RawEvent)
		wantKey string
	}{
		{
			name: "properties first level",
			mutate: func(raw *dto.RawEvent) {
				raw.Properties = map[string]any{"password": "hidden"}
			},
			wantKey: "password",
		},
		{
			name: "context first level",
			mutate: func(raw *dto.RawEvent) {
				raw.Context = map[string]any{"token": "hidden"}
			},
			wantKey: "token",
		},
		{
			name: "context nested map",
			mutate: func(raw *dto.RawEvent) {
				raw.Context = map[string]any{"runtime": map[string]any{"access_token": "hidden"}}
			},
			wantKey: "access_token",
		},
		{
			name: "context nested array",
			mutate: func(raw *dto.RawEvent) {
				raw.Context = map[string]any{"runtime": []any{map[string]any{"secret": "hidden"}}}
			},
			wantKey: "secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rejectedRepository := &fakeRejectedRepository{}
			eventRepository := &fakeEventRepository{}
			useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, &fakeDeduplicator{})
			raw := validRawEvent()
			tt.mutate(&raw)

			if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if len(eventRepository.events) != 0 {
				t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
			}
			if len(rejectedRepository.events) != 1 {
				t.Fatalf("rejected events = %d, want 1", len(rejectedRepository.events))
			}
			rejected := rejectedRepository.events[0]
			if rejected.Reason != rejection.ReasonBlockedKey {
				t.Fatalf("Reason = %q, want %q", rejected.Reason, rejection.ReasonBlockedKey)
			}
			if rejected.RawPayload["blocked_key"] != tt.wantKey {
				t.Fatalf("blocked_key = %#v, want %q", rejected.RawPayload["blocked_key"], tt.wantKey)
			}
		})
	}
}

func TestProcessRejectsPayloadLimitViolations(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*dto.RawEvent)
		wantIssue string
	}{
		{
			name: "context size",
			mutate: func(raw *dto.RawEvent) {
				raw.Context = map[string]any{"blob": strings.Repeat("x", maxPayloadObjectBytes+1)}
			},
			wantIssue: "context_size",
		},
		{
			name: "context depth",
			mutate: func(raw *dto.RawEvent) {
				value := map[string]any{"leaf": "ok"}
				for i := 0; i < maxPayloadDepth; i++ {
					value = map[string]any{"nested": value}
				}
				raw.Context = value
			},
			wantIssue: "context_depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rejectedRepository := &fakeRejectedRepository{}
			eventRepository := &fakeEventRepository{}
			useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, &fakeDeduplicator{})
			raw := validRawEvent()
			tt.mutate(&raw)

			if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if len(eventRepository.events) != 0 {
				t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
			}
			if len(rejectedRepository.events) != 1 {
				t.Fatalf("rejected events = %d, want 1", len(rejectedRepository.events))
			}
			rejected := rejectedRepository.events[0]
			if rejected.Reason != rejection.ReasonPayloadTooLarge {
				t.Fatalf("Reason = %q, want %q", rejected.Reason, rejection.ReasonPayloadTooLarge)
			}
			if rejected.RawPayload["payload_issue"] != tt.wantIssue {
				t.Fatalf("payload_issue = %#v, want %q", rejected.RawPayload["payload_issue"], tt.wantIssue)
			}
		})
	}
}
func TestProcessValidItemEventBuildsNormalizedDetails(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	eventRepository := &fakeEventRepository{}
	useCase := newTestProcessUseCase(now, eventRepository, &fakeRejectedRepository{}, cachedSite(), nil, &fakeDeduplicator{})
	raw := validRawEvent()
	raw.EventName = "item_impression"
	raw.Properties = map[string]any{
		"item_id":             "100",
		"surface":             "catalog",
		"list_instance_id":    "list_1",
		"impression_batch_id": "batch_1",
		"visible_ratio":       float64(75),
		"visible_time_ms":     float64(1500),
		"viewport_width":      float64(1366),
		"viewport_height":     float64(768),
		"ranking_run_id":      "rank_1",
		"ranking_version":     "v1",
		"item_type":           "product",
		"category_ids":        []any{"cat_1"},
	}

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
	details := eventRepository.events[0].ItemDetails
	if len(details) != 1 {
		t.Fatalf("ItemDetails = %d, want 1", len(details))
	}
	if details[0].TenantID != "tenant_123" || details[0].ItemID != "100" || details[0].Surface != "catalog" {
		t.Fatalf("detail = %#v, want tenant/item/surface", details[0])
	}
	if details[0].IncompleteForScoring {
		t.Fatalf("IncompleteForScoring = true, want false")
	}
}

func TestProcessMarksItemImpressionDedupKeyAfterPersistence(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	eventRepository := &fakeEventRepository{}
	deduplicator := &fakeDeduplicator{}
	useCase := newTestProcessUseCase(now, eventRepository, &fakeRejectedRepository{}, cachedSite(), nil, deduplicator)
	raw := validItemImpressionEvent()

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
	if !deduplicator.wasMarked(dedupStrategyItemImpression, "tenant_123:site_456:sess_xyz:catalog:list_1:100:") {
		t.Fatalf("marked = %#v, want item_impression key", deduplicator.marked)
	}
}

func TestProcessRejectsDuplicateItemImpressionBySpecificKey(t *testing.T) {
	rejectedRepository := &fakeRejectedRepository{}
	eventRepository := &fakeEventRepository{}
	deduplicator := &fakeDeduplicator{seenByStrategy: map[string]bool{dedupStrategyItemImpression: true}}
	useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, deduplicator)

	if err := useCase.Process(context.Background(), []dto.RawEvent{validItemImpressionEvent()}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 0 {
		t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
	}
	if len(rejectedRepository.events) != 1 {
		t.Fatalf("rejected events = %d, want 1", len(rejectedRepository.events))
	}
	rejected := rejectedRepository.events[0]
	if rejected.Reason != rejection.ReasonDuplicateSemanticEvent {
		t.Fatalf("Reason = %q, want %q", rejected.Reason, rejection.ReasonDuplicateSemanticEvent)
	}
	if rejected.RawPayload["dedup_strategy"] != dedupStrategyItemImpression {
		t.Fatalf("dedup_strategy = %#v, want item_impression", rejected.RawPayload["dedup_strategy"])
	}
}

func TestProcessRejectsDuplicatePurchaseLineBySpecificKey(t *testing.T) {
	rejectedRepository := &fakeRejectedRepository{}
	eventRepository := &fakeEventRepository{}
	deduplicator := &fakeDeduplicator{seenByStrategy: map[string]bool{dedupStrategyPurchaseLine: true}}
	useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, deduplicator)

	if err := useCase.Process(context.Background(), []dto.RawEvent{validPurchaseCompletedEvent()}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 0 {
		t.Fatalf("valid events = %d, want 0", len(eventRepository.events))
	}
	if len(rejectedRepository.events) != 1 {
		t.Fatalf("rejected events = %d, want 1", len(rejectedRepository.events))
	}
	rejected := rejectedRepository.events[0]
	if rejected.Reason != rejection.ReasonDuplicateSemanticEvent {
		t.Fatalf("Reason = %q, want %q", rejected.Reason, rejection.ReasonDuplicateSemanticEvent)
	}
	if rejected.RawPayload["dedup_strategy"] != dedupStrategyPurchaseLine {
		t.Fatalf("dedup_strategy = %#v, want purchase_line", rejected.RawPayload["dedup_strategy"])
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

func validItemImpressionEvent() dto.RawEvent {
	raw := validRawEvent()
	raw.EventName = "item_impression"
	raw.Properties = map[string]any{
		"item_id":             "100",
		"surface":             "catalog",
		"list_instance_id":    "list_1",
		"impression_batch_id": "batch_1",
		"visible_ratio":       float64(75),
		"visible_time_ms":     float64(1500),
		"viewport_width":      float64(1366),
		"viewport_height":     float64(768),
		"ranking_run_id":      "rank_1",
		"ranking_version":     "v1",
		"item_type":           "product",
		"category_ids":        []any{"cat_1"},
	}
	return raw
}

func validPurchaseCompletedEvent() dto.RawEvent {
	raw := validRawEvent()
	raw.EventName = "purchase_completed"
	raw.Properties = map[string]any{
		"order_id": "ord_1",
		"currency": "ARS",
		"items": []any{
			map[string]any{
				"item_id":       "100",
				"order_line_id": "line_1",
				"quantity":      float64(2),
				"unit_price":    float64(50),
				"currency":      "ARS",
				"item_type":     "product",
			},
		},
	}
	return raw
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

func (fake *fakeDeduplicator) wasMarked(strategy string, key string) bool {
	for _, marked := range fake.marked {
		if marked.Strategy == strategy && marked.Key == key {
			return true
		}
	}
	return false
}

type fakeClock struct {
	now time.Time
}

func (fake fakeClock) Now() time.Time {
	return fake.now
}
