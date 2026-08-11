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
func TestProcessPersistsRawContext(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	eventRepository := &fakeEventRepository{}
	useCase := newTestProcessUseCase(now, eventRepository, &fakeRejectedRepository{}, cachedSite(), nil, &fakeDeduplicator{})
	raw := validRawEvent()
	raw.Context = map[string]any{
		"app_area": "backoffice",
		"feature":  "catalog_search",
		"runtime": map[string]any{
			"sdk_mode": "manual",
		},
	}

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
	persisted := eventRepository.events[0]
	if persisted.Context["app_area"] != "backoffice" || persisted.Context["feature"] != "catalog_search" {
		t.Fatalf("context = %#v, want raw context preserved", persisted.Context)
	}
	runtime, ok := persisted.Context["runtime"].(map[string]any)
	if !ok || runtime["sdk_mode"] != "manual" {
		t.Fatalf("runtime context = %#v, want nested map preserved", persisted.Context["runtime"])
	}
}

func TestProcessKeepsItemSelectedForContextAsRawEvent(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	eventRepository := &fakeEventRepository{}
	useCase := newTestProcessUseCase(now, eventRepository, &fakeRejectedRepository{}, cachedSite(), nil, &fakeDeduplicator{})
	raw := validRawEvent()
	raw.EventName = "item_selected_for_context"
	raw.Properties = map[string]any{
		"item_id":    "100",
		"variant_id": "100-red-m",
		"item_type":  "product",
		"search_id":  "search_1",
		"position":   float64(1),
	}
	raw.Context = map[string]any{
		"feature":      "catalog_search",
		"surface":      "drawer",
		"component_id": "item_search",
	}

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(eventRepository.events))
	}
	persisted := eventRepository.events[0]
	if len(persisted.ItemDetails) != 0 {
		t.Fatalf("ItemDetails = %#v, want none for item_selected_for_context v1", persisted.ItemDetails)
	}
	if persisted.Properties["item_id"] != "100" || persisted.Context["component_id"] != "item_search" {
		t.Fatalf("persisted = %#v/%#v, want raw properties and context", persisted.Properties, persisted.Context)
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

func TestProcessAcceptsPersistentDuplicateAndRefreshesDeduplication(t *testing.T) {
	raw := validRawEvent()
	raw.LogicalEventID = "logical-duplicate"
	repository := &fakeEventRepository{status: outbound.EventPersistenceDuplicateLogical}
	deduplicator := &fakeDeduplicator{}
	useCase := newTestProcessUseCase(time.Now(), repository, &fakeRejectedRepository{}, cachedSite(), nil, deduplicator)

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !deduplicator.wasMarked(dedupStrategyLogical, scopedKey(cachedSite(), raw.LogicalEventID)) {
		t.Fatalf("duplicado persistente no refresco deduplicacion logica")
	}
}

func TestProcessPersistsContradictoryIdempotencyConflict(t *testing.T) {
	raw := validRawEvent()
	raw.IdempotencyKey = "operation-1"
	repository := &fakeEventRepository{results: []outbound.EventPersistenceResult{{
		EventID:         raw.EventID,
		Status:          outbound.EventPersistenceConflict,
		ExistingEventID: "existing-event",
		ConflictField:   "idempotency_key",
	}}}
	rejectedRepository := &fakeRejectedRepository{}
	useCase := newTestProcessUseCase(time.Now(), repository, rejectedRepository, cachedSite(), nil, &fakeDeduplicator{})

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(rejectedRepository.events) != 1 {
		t.Fatalf("rechazos = %d, want 1", len(rejectedRepository.events))
	}
	got := rejectedRepository.events[0]
	if got.Reason != rejection.ReasonIdempotencyConflict || got.RawPayload["conflict_field"] != "idempotency_key" || got.RawPayload["existing_event_id"] != "existing-event" {
		t.Fatalf("rechazo = %#v", got)
	}
}

func TestProcessPreservesSuccessfulResultsWhenAnotherEventFails(t *testing.T) {
	first := validRawEvent()
	second := validRawEvent()
	second.EventID = "event-second"
	second.LogicalEventID = "logical-second"
	repository := &fakeEventRepository{results: []outbound.EventPersistenceResult{
		{EventID: first.EventID, Err: errors.New("db caida")},
		{EventID: second.EventID, Status: outbound.EventPersistenceInserted},
	}}
	deduplicator := &fakeDeduplicator{}
	useCase := newTestProcessUseCase(time.Now(), repository, &fakeRejectedRepository{}, cachedSite(), nil, deduplicator)

	err := useCase.Process(context.Background(), []dto.RawEvent{first, second})
	if !errors.Is(err, ErrPersistValidEventsFailed) {
		t.Fatalf("error = %v, want ErrPersistValidEventsFailed", err)
	}
	if len(repository.events) != 2 {
		t.Fatalf("eventos entregados al repositorio = %d, want 2", len(repository.events))
	}
	if !deduplicator.wasMarked(dedupStrategyLogical, scopedKey(cachedSite(), second.LogicalEventID)) {
		t.Fatalf("resultado exitoso posterior no fue confirmado en deduplicacion")
	}
}

func TestRecordStreamRejectionStoresOnlySafePayloadMetadata(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	repository := &fakeRejectedRepository{}
	useCase := &ProcessEventsUseCase{rejectedRepository: repository, clock: fakeClock{now: now}}

	err := useCase.RecordStreamRejection(context.Background(), dto.StreamRejection{
		StreamID:      "1-0",
		EventID:       "evt-1",
		SiteCode:      "site-1",
		Environment:   "production",
		Reason:        rejection.ReasonInvalidStreamPayload,
		Payload:       `{"password":"no-persistir"}`,
		DeliveryCount: 2,
	})
	if err != nil {
		t.Fatalf("RecordStreamRejection() error = %v", err)
	}
	if len(repository.events) != 1 {
		t.Fatalf("rechazos = %d, want 1", len(repository.events))
	}
	got := repository.events[0]
	if got.EventID != "evt-1" || got.Reason != rejection.ReasonInvalidStreamPayload || !got.CreatedAt.Equal(now) {
		t.Fatalf("rechazo = %#v", got)
	}
	if _, exists := got.RawPayload["payload"]; exists {
		t.Fatalf("payload crudo no debe persistirse")
	}
	if got.RawPayload["payload_sha256"] == "" || got.RawPayload["payload_bytes"] != len([]byte(`{"password":"no-persistir"}`)) {
		t.Fatalf("metadata segura = %#v", got.RawPayload)
	}
}

func TestRecordStreamRejectionReturnsPersistenceError(t *testing.T) {
	repository := &fakeRejectedRepository{err: errors.New("postgres no disponible")}
	useCase := &ProcessEventsUseCase{rejectedRepository: repository, clock: fakeClock{now: time.Now()}}

	err := useCase.RecordStreamRejection(context.Background(), dto.StreamRejection{
		Reason:  rejection.ReasonStreamRetryExhausted,
		Payload: "{}",
	})
	if !errors.Is(err, ErrPersistRejectedFailed) {
		t.Fatalf("error = %v, want ErrPersistRejectedFailed", err)
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

func TestProcessPublishesPersistenceOutcomeMetric(t *testing.T) {
	now := time.Now()
	eventRepository := &fakeEventRepository{status: outbound.EventPersistenceDuplicateLogical}
	rejectedRepository := &fakeRejectedRepository{}
	cache := &fakeSiteCache{config: cachedSite(), found: true}
	resolver := &fakeSiteResolver{config: cachedSite()}
	deduplicator := &fakeDeduplicator{}
	metrics := &processMetricsFake{counters: map[string]int{}}
	useCase := NewProcessEventsUseCaseWithMetrics(
		nil, eventRepository, rejectedRepository, cache, resolver, deduplicator,
		fakeClock{now: now}, nil, RehydrateSiteOptions{}, nil, metrics,
	)

	if err := useCase.Process(context.Background(), []dto.RawEvent{validRawEvent()}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	key := outbound.MetricEventPersistence + ":duplicate_logical"
	if metrics.counters[key] != 1 {
		t.Fatalf("metricas = %#v", metrics.counters)
	}
}

func TestProcessPropagatesServerSourceSiteAndTimezone(t *testing.T) {
	now := time.Now()
	eventRepository := &fakeEventRepository{}
	rejectedRepository := &fakeRejectedRepository{}
	config := cachedSite()
	config.Timezone = "America/Argentina/Buenos_Aires"
	cache := &fakeSiteCache{config: config, found: true}
	useCase := newProcessUseCaseWithPorts(
		now, eventRepository, rejectedRepository, cache,
		&fakeSiteResolver{config: config}, &fakeDeduplicator{},
	)
	raw := validRawEvent()
	raw.EventSource = site.SourceServer
	raw.TokenVersion = 0
	raw.JWTID = ""
	raw.AnonymousID = ""
	raw.SessionID = ""
	raw.Origin = ""
	raw.URL = ""
	raw.Path = ""

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(rejectedRepository.events) != 0 || len(eventRepository.events) != 1 {
		t.Fatalf("persistidos=%d rechazados=%d", len(eventRepository.events), len(rejectedRepository.events))
	}
	got := eventRepository.events[0]
	if got.EventSource != site.SourceServer || got.TenantID != config.TenantID || got.SiteID != config.SiteID || got.Timezone != config.Timezone {
		t.Fatalf("trazabilidad propagada = %#v", got)
	}
	metadata, ok := got.Context["_goanalytics"].(map[string]any)
	if !ok || metadata["event_source"] != site.SourceServer || metadata["timezone"] != config.Timezone {
		t.Fatalf("snapshot interno = %#v", got.Context["_goanalytics"])
	}
	if len(cache.keys) != 1 || cache.keys[0].Source != site.SourceServer || cache.keys[0].Origin != "" {
		t.Fatalf("clave de resolución = %#v", cache.keys)
	}
}

func TestProcessDoesNotRelaxWebValidation(t *testing.T) {
	eventRepository := &fakeEventRepository{}
	rejectedRepository := &fakeRejectedRepository{}
	useCase := newTestProcessUseCase(time.Now(), eventRepository, rejectedRepository, cachedSite(), nil, &fakeDeduplicator{})
	raw := validRawEvent()
	raw.EventSource = site.SourceWeb
	raw.Origin = ""

	if err := useCase.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(eventRepository.events) != 0 || len(rejectedRepository.events) != 1 || rejectedRepository.events[0].Reason != rejection.ReasonInvalidPayload {
		t.Fatalf("persistidos=%d rechazos=%#v", len(eventRepository.events), rejectedRepository.events)
	}
}

func TestProcessSubjectNormalizationIsOptIn(t *testing.T) {
	t.Parallel()

	raw := validRawEvent()
	raw.EventName = "subject_selected"
	raw.Properties = map[string]any{"subjects": []any{map[string]any{
		"subject_type": "customer", "subject_id": "customer-1", "relation": "selected",
	}}}
	config := cachedSite()
	config.Timezone = "America/Argentina/Buenos_Aires"

	disabledRepository := &fakeEventRepository{}
	disabled := newTestProcessUseCase(time.Now(), disabledRepository, &fakeRejectedRepository{}, config, nil, &fakeDeduplicator{})
	if err := disabled.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process(disabled) error = %v", err)
	}
	if len(disabledRepository.events) != 1 || disabledRepository.events[0].SubjectDetails != nil {
		t.Fatalf("normalización apagada = %#v", disabledRepository.events)
	}

	enabledRepository := &fakeEventRepository{}
	enabled := NewProcessEventsUseCaseWithFeatures(
		nil, enabledRepository, &fakeRejectedRepository{},
		&fakeSiteCache{config: config, found: true}, &fakeSiteResolver{config: config},
		&fakeDeduplicator{}, fakeClock{now: time.Now()}, nil,
		RehydrateSiteOptions{}, nil, nil,
		ProcessFeatures{SubjectNormalizationEnabled: true},
	)
	if err := enabled.Process(context.Background(), []dto.RawEvent{raw}); err != nil {
		t.Fatalf("Process(enabled) error = %v", err)
	}
	if len(enabledRepository.events) != 1 || len(enabledRepository.events[0].SubjectDetails) != 1 {
		t.Fatalf("normalización encendida = %#v", enabledRepository.events)
	}
}

type processMetricsFake struct {
	counters map[string]int
}

func (fake *processMetricsFake) IncrementCounter(name string, labels map[string]string) {
	fake.counters[name+":"+labels["outcome"]]++
}

func (fake *processMetricsFake) SetGauge(_ string, _ float64, _ map[string]string) {}

func (fake *processMetricsFake) ObserveHistogram(_ string, _ float64, _ map[string]string) {}

func validRawEvent() dto.RawEvent {
	eventTime := time.Date(2026, 5, 5, 11, 59, 0, 0, time.UTC)
	receivedAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	return dto.RawEvent{
		EventID:      "evt_1",
		EventSource:  site.SourceWeb,
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
	events  []event.ValidatedEvent
	err     error
	status  outbound.EventPersistenceStatus
	results []outbound.EventPersistenceResult
}

func (fake *fakeEventRepository) SaveBatch(ctx context.Context, events []event.ValidatedEvent) []outbound.EventPersistenceResult {
	_ = ctx
	fake.events = append(fake.events, events...)
	if fake.results != nil {
		return fake.results
	}
	status := fake.status
	if status == "" {
		status = outbound.EventPersistenceInserted
	}
	results := make([]outbound.EventPersistenceResult, 0, len(events))
	for _, item := range events {
		results = append(results, outbound.EventPersistenceResult{EventID: item.EventID, Status: status, Err: fake.err})
	}
	return results
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
