package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/ingest/internal/application/dto"
	"goanalytics/services/ingest/internal/domain/event"
	"goanalytics/services/ingest/internal/domain/token"
)

func TestIngestValidPublishesEnrichedEvents(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	publisher := &fakePublisher{}
	limiter := &fakeRateLimiter{allow: true}
	useCase := newTestUseCase(now, publisher, limiter, nil)

	response, err := useCase.Ingest(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if response.Accepted != 1 {
		t.Fatalf("Accepted = %d, want 1", response.Accepted)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}

	published := publisher.events[0]
	if published.ReceivedAt != now {
		t.Fatalf("ReceivedAt = %v, want %v", published.ReceivedAt, now)
	}
	if published.UserAgent != "Mozilla/5.0" {
		t.Fatalf("UserAgent = %q, want Mozilla/5.0", published.UserAgent)
	}
	if published.IPHash != "ip_hash_123" {
		t.Fatalf("IPHash = %q, want ip_hash_123", published.IPHash)
	}
	if published.SitePublicID != "pub_site_abc123" || published.Environment != "production" {
		t.Fatalf("claims de site = %q/%q, want pub_site_abc123/production", published.SitePublicID, published.Environment)
	}
	if published.JWTID != "jti_123" || published.TokenVersion != 1 {
		t.Fatalf("claims de token = %q/%d, want jti_123/1", published.JWTID, published.TokenVersion)
	}
	if published.SDKName != "goanalytics-web" || published.SDKVersion != "1.0.0" {
		t.Fatalf("SDK metadata = %q/%q, want goanalytics-web/1.0.0", published.SDKName, published.SDKVersion)
	}
	if published.Properties == nil || published.Context == nil {
		t.Fatalf("Properties y Context deben publicarse como mapas no nulos")
	}
	if len(limiter.calls) != 2 {
		t.Fatalf("rate limiter calls = %d, want 2", len(limiter.calls))
	}
}

func TestIngestInvalidTokenStopsBeforeLimitsAndPublish(t *testing.T) {
	publisher := &fakePublisher{}
	limiter := &fakeRateLimiter{allow: true}
	useCase := newTestUseCase(time.Now(), publisher, limiter, errors.New("firma invalida"))

	_, err := useCase.Ingest(context.Background(), validRequest())
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Ingest() error = %v, want ErrInvalidToken", err)
	}
	if len(limiter.calls) != 0 {
		t.Fatalf("rate limiter calls = %d, want 0", len(limiter.calls))
	}
	if len(publisher.events) != 0 {
		t.Fatalf("published events = %d, want 0", len(publisher.events))
	}
}

func TestIngestRejectsInvalidClaimsAsInvalidToken(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	claims := validClaims(now)
	claims.SitePublicID = ""
	useCase := newUseCaseWithClaims(now, &fakePublisher{}, &fakeRateLimiter{allow: true}, claims, nil)

	_, err := useCase.Ingest(context.Background(), validRequest())
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Ingest() error = %v, want ErrInvalidToken", err)
	}
}

func TestIngestRejectsEmptyAndOversizedBatch(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		request dto.IngestRequest
	}{
		{name: "empty", request: dto.IngestRequest{Token: "token"}},
		{name: "oversized", request: oversizedRequest(2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &fakePublisher{}
			useCase := newUseCaseWithOptions(now, publisher, &fakeRateLimiter{allow: true}, nil, IngestOptions{
				MaxEventsPerBatch: 1,
				SiteRateLimit:     100,
				IPRateLimit:       100,
				RateLimitWindow:   time.Minute,
			})

			_, err := useCase.Ingest(context.Background(), tt.request)
			if !errors.Is(err, ErrInvalidBatch) {
				t.Fatalf("Ingest() error = %v, want ErrInvalidBatch", err)
			}
			if len(publisher.events) != 0 {
				t.Fatalf("published events = %d, want 0", len(publisher.events))
			}
		})
	}
}

func TestIngestRejectsInvalidEventDoesNotPublish(t *testing.T) {
	request := validRequest()
	request.Events[0].EventName = ""
	publisher := &fakePublisher{}
	useCase := newTestUseCase(time.Now(), publisher, &fakeRateLimiter{allow: true}, nil)

	_, err := useCase.Ingest(context.Background(), request)
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("Ingest() error = %v, want ErrInvalidPayload", err)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("published events = %d, want 0", len(publisher.events))
	}
}

func TestIngestRateLimitExceededDoesNotPublish(t *testing.T) {
	publisher := &fakePublisher{}
	useCase := newTestUseCase(time.Now(), publisher, &fakeRateLimiter{allow: false}, nil)

	_, err := useCase.Ingest(context.Background(), validRequest())
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("Ingest() error = %v, want ErrRateLimitExceeded", err)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("published events = %d, want 0", len(publisher.events))
	}
}

func TestIngestPublisherErrorIsWrapped(t *testing.T) {
	publisher := &fakePublisher{err: errors.New("stream no disponible")}
	useCase := newTestUseCase(time.Now(), publisher, &fakeRateLimiter{allow: true}, nil)

	_, err := useCase.Ingest(context.Background(), validRequest())
	if !errors.Is(err, ErrPublishFailed) {
		t.Fatalf("Ingest() error = %v, want ErrPublishFailed", err)
	}
}

func newTestUseCase(now time.Time, publisher *fakePublisher, limiter *fakeRateLimiter, verifierErr error) *IngestEventsUseCase {
	return newUseCaseWithClaims(now, publisher, limiter, validClaims(now), verifierErr)
}

func newUseCaseWithClaims(now time.Time, publisher *fakePublisher, limiter *fakeRateLimiter, claims token.TrackingClaims, verifierErr error) *IngestEventsUseCase {
	return newUseCaseWithOptions(now, publisher, limiter, verifierErr, IngestOptions{
		MaxEventsPerBatch: 10,
		SiteRateLimit:     100,
		IPRateLimit:       100,
		RateLimitWindow:   time.Minute,
		SDKName:           "goanalytics-web",
		SDKVersion:        "1.0.0",
	}, claims)
}

func newUseCaseWithOptions(now time.Time, publisher *fakePublisher, limiter *fakeRateLimiter, verifierErr error, options IngestOptions, claimsOverride ...token.TrackingClaims) *IngestEventsUseCase {
	claims := validClaims(now)
	if len(claimsOverride) > 0 {
		claims = claimsOverride[0]
	}
	return NewIngestEventsUseCase(
		&fakeTokenVerifier{claims: claims, err: verifierErr},
		publisher,
		limiter,
		fakeClock{now: now},
		nil,
		options,
	)
}

func validRequest() dto.IngestRequest {
	timestamp := time.Date(2026, 5, 5, 11, 59, 0, 0, time.UTC)
	return dto.IngestRequest{
		Token:     "token",
		IPHash:    "ip_hash_123",
		UserAgent: "Mozilla/5.0",
		Events: []dto.IngestEvent{
			{
				EventID:      "018f9b8e-0000-7000-a000-000000000001",
				EventName:    "page_view",
				EventVersion: 1,
				Timestamp:    timestamp,
				AnonymousID:  "anon_abc",
				SessionID:    "sess_xyz",
				Origin:       "https://cliente.com",
				URL:          "https://cliente.com/productos/123",
				Path:         "/productos/123",
			},
		},
	}
}

func oversizedRequest(size int) dto.IngestRequest {
	request := validRequest()
	eventItem := request.Events[0]
	request.Events = make([]dto.IngestEvent, 0, size)
	for i := 0; i < size; i++ {
		request.Events = append(request.Events, eventItem)
	}
	return request
}

func validClaims(now time.Time) token.TrackingClaims {
	return token.TrackingClaims{
		Issuer:       "main-backend",
		Audience:     "analytics-ingest",
		SitePublicID: "pub_site_abc123",
		Environment:  "production",
		TokenVersion: 1,
		IssuedAt:     now.Add(-time.Minute),
		NotBefore:    now.Add(-time.Minute),
		ExpiresAt:    now.Add(time.Minute),
		JWTID:        "jti_123",
	}
}

type fakeTokenVerifier struct {
	claims token.TrackingClaims
	err    error
}

func (fake fakeTokenVerifier) Verify(ctx context.Context, rawToken string) (token.TrackingClaims, error) {
	_ = ctx
	_ = rawToken
	return fake.claims, fake.err
}

type fakePublisher struct {
	events []event.RawEvent
	err    error
}

func (fake *fakePublisher) Publish(ctx context.Context, events []event.RawEvent) error {
	_ = ctx
	fake.events = append(fake.events, events...)
	return fake.err
}

type fakeRateLimitCall struct {
	key    string
	limit  int
	window time.Duration
}

type fakeRateLimiter struct {
	calls []fakeRateLimitCall
	allow bool
	err   error
}

func (fake *fakeRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	_ = ctx
	fake.calls = append(fake.calls, fakeRateLimitCall{key: key, limit: limit, window: window})
	return fake.allow, fake.err
}

type fakeClock struct {
	now time.Time
}

func (fake fakeClock) Now() time.Time {
	return fake.now
}
