package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	consumerMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type subscriptionPaidEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *subscriptionPaidEvent) GetEventType() string { return e.eventType }
func (e *subscriptionPaidEvent) GetPayload() any      { return e.envelope }

func newSubscriptionPaidEvent(payload any) events.Event {
	rawPayload, _ := json.Marshal(payload)
	return &subscriptionPaidEvent{
		eventType: "billing.subscription.activated",
		envelope: outbox.Envelope{
			ID:        "evt-001",
			EventType: "billing.subscription.activated",
			Payload:   json.RawMessage(rawPayload),
		},
	}
}

type subscriptionPaidBadPayloadEvent struct{}

func (e *subscriptionPaidBadPayloadEvent) GetEventType() string {
	return "billing.subscription.activated"
}
func (e *subscriptionPaidBadPayloadEvent) GetPayload() any { return "not-an-envelope" }

type SubscriptionPaidConsumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSubscriptionPaidConsumerSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionPaidConsumerSuite))
}

func (s *SubscriptionPaidConsumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubscriptionPaidConsumerSuite) TestHandle() {
	paidAt := time.Now().UTC().Truncate(time.Second)

	type args struct {
		event events.Event
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*consumerMocks.MarkTokenPaidUseCase)
		expect func(error)
	}{
		{
			name: "deve chamar mark token paid",
			args: args{
				event: newSubscriptionPaidEvent(map[string]any{
					"subscription_id":      "sub-001",
					"funnel_token":         "token-sck-abc",
					"plan_code":            "MONTHLY",
					"external_sale_id":     "sale-001",
					"customer_mobile_e164": "+5511999999999",
					"customer_email":       "user@example.com",
					"paid_at":              paidAt,
					"occurred_at":          paidAt,
				}),
			},
			setup: func(useCase *consumerMocks.MarkTokenPaidUseCase) {
				useCase.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.MarkTokenPaidInput) bool {
					return in.SubscriptionID == "sub-001" &&
						in.FunnelToken == "token-sck-abc" &&
						in.CustomerMobileE164 == "+5511999999999" &&
						in.CustomerEmail == "user@example.com" &&
						in.ExternalSaleID == "sale-001"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve ignorar payload sem funnel token",
			args: args{
				event: newSubscriptionPaidEvent(map[string]any{
					"subscription_id":      "sub-002",
					"funnel_token":         "",
					"external_sale_id":     "sale-002",
					"customer_mobile_e164": "+5511999999999",
					"customer_email":       "user@example.com",
					"paid_at":              paidAt,
				}),
			},
			setup: func(useCase *consumerMocks.MarkTokenPaidUseCase) {},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do use case",
			args: args{
				event: newSubscriptionPaidEvent(map[string]any{
					"funnel_token":     "token-err",
					"external_sale_id": "sale-err",
					"paid_at":          paidAt,
				}),
			},
			setup: func(useCase *consumerMocks.MarkTokenPaidUseCase) {
				useCase.EXPECT().Execute(mock.Anything, mock.Anything).Return(errors.New("db error")).Once()
			},
			expect: func(err error) {
				s.ErrorContains(err, "mark token paid")
			},
		},
		{
			name:  "deve retornar erro para payload inesperado",
			args:  args{event: &subscriptionPaidBadPayloadEvent{}},
			setup: func(useCase *consumerMocks.MarkTokenPaidUseCase) {},
			expect: func(err error) {
				s.ErrorContains(err, "unexpected payload type")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			useCase := consumerMocks.NewMarkTokenPaidUseCase(s.T())
			scenario.setup(useCase)
			consumer := consumers.NewSubscriptionPaidConsumer(useCase, noop.NewProvider())
			scenario.expect(consumer.Handle(s.ctx, scenario.args.event))
		})
	}
}
