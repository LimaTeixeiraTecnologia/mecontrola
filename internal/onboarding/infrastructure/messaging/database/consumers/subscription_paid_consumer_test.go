package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type mockMarkTokenPaid struct {
	mock.Mock
}

func (m *mockMarkTokenPaid) Execute(ctx context.Context, in input.MarkTokenPaidInput) error {
	return m.Called(ctx, in).Error(0)
}

type fakeEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *fakeEvent) GetEventType() string { return e.eventType }
func (e *fakeEvent) GetPayload() any      { return e.envelope }

func makeActivatedEnvelope(payload any) events.Event {
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{
		ID:        "evt-001",
		EventType: "billing.subscription.activated",
		Payload:   json.RawMessage(raw),
	}
	return &fakeEvent{eventType: "billing.subscription.activated", envelope: env}
}

type SubscriptionPaidConsumerSuite struct {
	suite.Suite
	usecase  *mockMarkTokenPaid
	consumer *consumers.SubscriptionPaidConsumer
}

func TestSubscriptionPaidConsumer(t *testing.T) {
	suite.Run(t, new(SubscriptionPaidConsumerSuite))
}

func (s *SubscriptionPaidConsumerSuite) SetupTest() {
	s.usecase = &mockMarkTokenPaid{}
	s.consumer = consumers.NewSubscriptionPaidConsumer(s.usecase, noop.NewProvider())
}

func (s *SubscriptionPaidConsumerSuite) TestHandleCallsMarkTokenPaid() {
	paidAt := time.Now().UTC().Truncate(time.Second)
	payload := map[string]any{
		"subscription_id":      "sub-001",
		"funnel_token":         "token-sck-abc",
		"plan_code":            "MONTHLY",
		"external_sale_id":     "sale-001",
		"customer_mobile_e164": "+5511999999999",
		"customer_email":       "user@example.com",
		"paid_at":              paidAt,
		"occurred_at":          paidAt,
	}

	s.usecase.On("Execute", mock.Anything, mock.MatchedBy(func(in input.MarkTokenPaidInput) bool {
		return in.SubscriptionID == "sub-001" &&
			in.FunnelToken == "token-sck-abc" &&
			in.CustomerMobileE164 == "+5511999999999" &&
			in.CustomerEmail == "user@example.com" &&
			in.ExternalSaleID == "sale-001"
	})).Return(nil)

	err := s.consumer.Handle(context.Background(), makeActivatedEnvelope(payload))
	s.Require().NoError(err)
	s.usecase.AssertExpectations(s.T())
}

func (s *SubscriptionPaidConsumerSuite) TestHandleNoopWhenFunnelTokenEmpty() {
	paidAt := time.Now().UTC()
	payload := map[string]any{
		"subscription_id":      "sub-002",
		"funnel_token":         "",
		"external_sale_id":     "sale-002",
		"customer_mobile_e164": "+5511999999999",
		"customer_email":       "user@example.com",
		"paid_at":              paidAt,
	}

	err := s.consumer.Handle(context.Background(), makeActivatedEnvelope(payload))
	s.Require().NoError(err)
	s.usecase.AssertNotCalled(s.T(), "Execute")
}

func (s *SubscriptionPaidConsumerSuite) TestHandleIdempotentRetry() {
	paidAt := time.Now().UTC()
	payload := map[string]any{
		"funnel_token":         "token-idem",
		"external_sale_id":     "sale-idem",
		"customer_mobile_e164": "+5511999999999",
		"customer_email":       "user@example.com",
		"paid_at":              paidAt,
	}

	s.usecase.On("Execute", mock.Anything, mock.Anything).Return(nil)

	evt := makeActivatedEnvelope(payload)
	err1 := s.consumer.Handle(context.Background(), evt)
	s.Require().NoError(err1)
	err2 := s.consumer.Handle(context.Background(), evt)
	s.Require().NoError(err2)
}

func (s *SubscriptionPaidConsumerSuite) TestHandleUsecaseErrorPropagated() {
	paidAt := time.Now().UTC()
	payload := map[string]any{
		"funnel_token":     "token-err",
		"external_sale_id": "sale-err",
		"paid_at":          paidAt,
	}

	ucErr := errors.New("db error")
	s.usecase.On("Execute", mock.Anything, mock.Anything).Return(ucErr)

	err := s.consumer.Handle(context.Background(), makeActivatedEnvelope(payload))
	s.Require().Error(err)
	s.ErrorContains(err, "mark token paid")
}

func (s *SubscriptionPaidConsumerSuite) TestHandleUnexpectedPayloadTypeReturnsError() {
	badEvt := &badPayloadEvent{}
	err := s.consumer.Handle(context.Background(), badEvt)
	s.Require().Error(err)
	s.ErrorContains(err, "unexpected payload type")
}

type badPayloadEvent struct{}

func (e *badPayloadEvent) GetEventType() string { return "billing.subscription.activated" }
func (e *badPayloadEvent) GetPayload() any      { return "not-an-envelope" }
