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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type mockHandlePaidWithoutToken struct {
	mock.Mock
}

func (m *mockHandlePaidWithoutToken) Execute(ctx context.Context, in input.HandlePaidWithoutTokenInput) error {
	return m.Called(ctx, in).Error(0)
}

func makeWithoutTokenEnvelope(payload any) *fakeEvent {
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{
		ID:        "evt-nowt-001",
		EventType: "billing.subscription.activated_without_token",
		Payload:   json.RawMessage(raw),
	}
	return &fakeEvent{eventType: "billing.subscription.activated_without_token", envelope: env}
}

type PaidWithoutTokenConsumerSuite struct {
	suite.Suite
	usecase  *mockHandlePaidWithoutToken
	consumer *consumers.PaidWithoutTokenConsumer
}

func TestPaidWithoutTokenConsumer(t *testing.T) {
	suite.Run(t, new(PaidWithoutTokenConsumerSuite))
}

func (s *PaidWithoutTokenConsumerSuite) SetupTest() {
	s.usecase = &mockHandlePaidWithoutToken{}
	s.consumer = consumers.NewPaidWithoutTokenConsumer(s.usecase, noop.NewProvider())
}

func (s *PaidWithoutTokenConsumerSuite) TestHandleCallsUseCase() {
	paidAt := time.Now().UTC().Truncate(time.Second)
	payload := map[string]any{
		"subscription_id":      "sub-nowt-001",
		"external_sale_id":     "sale-nowt-001",
		"customer_mobile_e164": "+5511999888888",
		"customer_email":       "user@example.com",
		"paid_at":              paidAt,
		"occurred_at":          paidAt,
	}

	s.usecase.On("Execute", mock.Anything, mock.MatchedBy(func(in input.HandlePaidWithoutTokenInput) bool {
		return in.ExternalSaleID == "sale-nowt-001" &&
			in.CustomerMobileE164 == "+5511999888888" &&
			in.CustomerEmail == "user@example.com"
	})).Return(nil)

	err := s.consumer.Handle(context.Background(), makeWithoutTokenEnvelope(payload))
	s.Require().NoError(err)
	s.usecase.AssertExpectations(s.T())
}

func (s *PaidWithoutTokenConsumerSuite) TestHandleUsecaseErrorPropagated() {
	paidAt := time.Now().UTC()
	payload := map[string]any{
		"external_sale_id":     "sale-err",
		"customer_mobile_e164": "+5511999999999",
		"customer_email":       "user@example.com",
		"paid_at":              paidAt,
	}

	ucErr := errors.New("signal insert failed")
	s.usecase.On("Execute", mock.Anything, mock.Anything).Return(ucErr)

	err := s.consumer.Handle(context.Background(), makeWithoutTokenEnvelope(payload))
	s.Require().Error(err)
	s.ErrorContains(err, "handle")
}

func (s *PaidWithoutTokenConsumerSuite) TestHandleUnexpectedPayloadTypeReturnsError() {
	err := s.consumer.Handle(context.Background(), &badPayloadEvent{})
	s.Require().Error(err)
	s.ErrorContains(err, "unexpected payload type")
}

func (s *PaidWithoutTokenConsumerSuite) TestHandleSupportSignalCreatedForPaidWithoutToken() {
	paidAt := time.Now().UTC()
	payload := map[string]any{
		"external_sale_id":     "sale-support-001",
		"customer_mobile_e164": "+5511999777777",
		"customer_email":       "support@example.com",
		"paid_at":              paidAt,
	}

	s.usecase.On("Execute", mock.Anything, mock.MatchedBy(func(in input.HandlePaidWithoutTokenInput) bool {
		return in.ExternalSaleID == "sale-support-001"
	})).Return(nil)

	err := s.consumer.Handle(context.Background(), makeWithoutTokenEnvelope(payload))
	s.Require().NoError(err)
}
