package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type mockEntitlementRepo struct {
	mock.Mock
}

func (m *mockEntitlementRepo) Upsert(ctx context.Context, record interfaces.EntitlementRecord) error {
	return m.Called(ctx, record).Error(0)
}

func (m *mockEntitlementRepo) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(interfaces.EntitlementRecord), args.Error(1)
}

func (m *mockEntitlementRepo) UpsertPending(ctx context.Context, subscriptionID string, funnelToken string, payload []byte) error {
	return m.Called(ctx, subscriptionID, funnelToken, payload).Error(0)
}

type mockRepositoryFactory struct {
	mock.Mock
}

func (f *mockRepositoryFactory) UserRepository(db database.DBTX) interfaces.UserRepository {
	return nil
}

func (f *mockRepositoryFactory) EntitlementRepository(db database.DBTX) interfaces.EntitlementRepository {
	args := f.Called(db)
	return args.Get(0).(interfaces.EntitlementRepository)
}

type mockProjectionReader struct {
	mock.Mock
}

func (m *mockProjectionReader) FindCurrentBySubscriptionID(ctx context.Context, subscriptionID string) (interfaces.SubscriptionProjectionRecord, error) {
	args := m.Called(ctx, subscriptionID)
	return args.Get(0).(interfaces.SubscriptionProjectionRecord), args.Error(1)
}

type SubscriptionEventProjectorSuite struct {
	suite.Suite
	factory          *mockRepositoryFactory
	entRepo          *mockEntitlementRepo
	projectionReader *mockProjectionReader
	projector        *consumers.SubscriptionEventProjector
}

func TestSubscriptionEventProjector(t *testing.T) {
	suite.Run(t, new(SubscriptionEventProjectorSuite))
}

func (s *SubscriptionEventProjectorSuite) SetupTest() {
	s.factory = &mockRepositoryFactory{}
	s.entRepo = &mockEntitlementRepo{}
	s.projectionReader = &mockProjectionReader{}
	uc := usecases.NewProjectSubscriptionEvent(s.factory, nil, s.projectionReader, noop.NewProvider())
	s.projector = consumers.NewSubscriptionEventProjector(uc, noop.NewProvider())
}

func makeEnvelope(eventType string, payload any) events.Event {
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{
		ID:        "test-id",
		EventType: eventType,
		Payload:   json.RawMessage(raw),
	}
	return &fakeEvent{eventType: eventType, envelope: env}
}

type fakeEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *fakeEvent) GetEventType() string { return e.eventType }
func (e *fakeEvent) GetPayload() any      { return e.envelope }

func (s *SubscriptionEventProjectorSuite) TestActivatedWithNoUserIDGoesToPending() {
	payload := map[string]any{
		"subscription_id": "sub-123",
		"funnel_token":    "token-abc",
		"plan_code":       "MONTHLY",
		"period_start":    time.Now().UTC(),
		"period_end":      time.Now().UTC().Add(30 * 24 * time.Hour),
		"occurred_at":     time.Now().UTC(),
	}

	s.factory.On("EntitlementRepository", mock.Anything).Return(s.entRepo)
	s.entRepo.On("UpsertPending", mock.Anything, "sub-123", "token-abc", mock.Anything).Return(nil)

	s.projectionReader.On("FindCurrentBySubscriptionID", mock.Anything, "sub-123").Return(interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-123",
		FunnelToken:    "token-abc",
		Status:         "ACTIVE",
		PeriodEnd:      payload["period_end"].(time.Time),
		OccurredAt:     payload["occurred_at"].(time.Time),
	}, nil)

	err := s.projector.Handle(context.Background(), makeEnvelope("billing.subscription.activated", payload))
	s.Require().NoError(err)
	s.entRepo.AssertCalled(s.T(), "UpsertPending", mock.Anything, "sub-123", "token-abc", mock.Anything)
}

func (s *SubscriptionEventProjectorSuite) TestActivatedIdempotentSecondCall() {
	payload := map[string]any{
		"subscription_id": "sub-idem",
		"funnel_token":    "token-idem",
		"plan_code":       "MONTHLY",
		"period_start":    time.Now().UTC(),
		"period_end":      time.Now().UTC().Add(30 * 24 * time.Hour),
		"occurred_at":     time.Now().UTC(),
	}

	s.factory.On("EntitlementRepository", mock.Anything).Return(s.entRepo)
	s.entRepo.On("UpsertPending", mock.Anything, "sub-idem", "token-idem", mock.Anything).Return(nil)

	s.projectionReader.On("FindCurrentBySubscriptionID", mock.Anything, "sub-idem").Return(interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-idem",
		FunnelToken:    "token-idem",
		Status:         "ACTIVE",
		PeriodEnd:      payload["period_end"].(time.Time),
		OccurredAt:     payload["occurred_at"].(time.Time),
	}, nil)

	event := makeEnvelope("billing.subscription.activated", payload)

	err1 := s.projector.Handle(context.Background(), event)
	s.Require().NoError(err1)

	err2 := s.projector.Handle(context.Background(), event)
	s.Require().NoError(err2)
}

func (s *SubscriptionEventProjectorSuite) TestPastDueWithNoUserUpdatesPending() {
	payload := map[string]any{
		"subscription_id": "sub-pd",
		"period_end":      time.Now().UTC().Add(-time.Hour),
		"grace_end":       time.Now().UTC().Add(2 * 24 * time.Hour),
		"occurred_at":     time.Now().UTC(),
	}

	s.projectionReader.On("FindCurrentBySubscriptionID", mock.Anything, "sub-pd").Return(interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-pd",
		FunnelToken:    "token-pd",
		Status:         "REFUNDED",
		PeriodEnd:      payload["period_end"].(time.Time),
		OccurredAt:     payload["occurred_at"].(time.Time).Add(time.Hour),
	}, nil)
	s.factory.On("EntitlementRepository", mock.Anything).Return(s.entRepo)
	s.entRepo.On("UpsertPending", mock.Anything, "sub-pd", "token-pd", mock.MatchedBy(func(raw []byte) bool {
		var current map[string]any
		return json.Unmarshal(raw, &current) == nil && current["status"] == "REFUNDED"
	})).Return(nil)

	err := s.projector.Handle(context.Background(), makeEnvelope("billing.subscription.past_due", payload))
	s.Require().NoError(err)
	s.entRepo.AssertCalled(s.T(), "UpsertPending", mock.Anything, "sub-pd", "token-pd", mock.Anything)
}

func (s *SubscriptionEventProjectorSuite) TestUnknownEventTypeIsNoOp() {
	env := outbox.Envelope{
		ID:        "test-unknown",
		EventType: "billing.subscription.unknown",
		Payload:   json.RawMessage(`{}`),
	}
	event := &fakeEvent{eventType: "billing.subscription.unknown", envelope: env}
	err := s.projector.Handle(context.Background(), event)
	s.Require().NoError(err)
}
