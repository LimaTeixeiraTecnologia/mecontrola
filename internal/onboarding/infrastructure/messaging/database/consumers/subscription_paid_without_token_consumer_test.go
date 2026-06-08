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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

func newPaidWithoutTokenEvent(payload any) *subscriptionPaidEvent {
	rawPayload, _ := json.Marshal(payload)
	return &subscriptionPaidEvent{
		eventType: "billing.subscription.activated_without_token",
		envelope: outbox.Envelope{
			ID:        "evt-nowt-001",
			EventType: "billing.subscription.activated_without_token",
			Payload:   json.RawMessage(rawPayload),
		},
	}
}

type PaidWithoutTokenConsumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestPaidWithoutTokenConsumerSuite(t *testing.T) {
	suite.Run(t, new(PaidWithoutTokenConsumerSuite))
}

func (s *PaidWithoutTokenConsumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *PaidWithoutTokenConsumerSuite) TestHandle() {
	paidAt := time.Now().UTC().Truncate(time.Second)

	type args struct {
		event *subscriptionPaidEvent
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*consumerMocks.HandlePaidWithoutTokenUseCase)
		expect func(error)
	}{
		{
			name: "deve chamar use case",
			args: args{
				event: newPaidWithoutTokenEvent(map[string]any{
					"subscription_id":      "sub-nowt-001",
					"external_sale_id":     "sale-nowt-001",
					"customer_mobile_e164": "+5511999888888",
					"customer_email":       "user@example.com",
					"paid_at":              paidAt,
					"occurred_at":          paidAt,
				}),
			},
			setup: func(useCase *consumerMocks.HandlePaidWithoutTokenUseCase) {
				useCase.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.HandlePaidWithoutTokenInput) bool {
					return in.ExternalSaleID == "sale-nowt-001" &&
						in.CustomerMobileE164 == "+5511999888888" &&
						in.CustomerEmail == "user@example.com"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do use case",
			args: args{
				event: newPaidWithoutTokenEvent(map[string]any{
					"external_sale_id":     "sale-err",
					"customer_mobile_e164": "+5511999999999",
					"customer_email":       "user@example.com",
					"paid_at":              paidAt,
				}),
			},
			setup: func(useCase *consumerMocks.HandlePaidWithoutTokenUseCase) {
				useCase.EXPECT().Execute(mock.Anything, mock.Anything).Return(errors.New("signal insert failed")).Once()
			},
			expect: func(err error) {
				s.ErrorContains(err, "handle")
			},
		},
		{
			name:  "deve retornar erro para payload inesperado",
			setup: func(useCase *consumerMocks.HandlePaidWithoutTokenUseCase) {},
			expect: func(err error) {
				s.ErrorContains(err, "unexpected payload type")
			},
		},
		{
			name: "deve aceitar payload de suporte",
			args: args{
				event: newPaidWithoutTokenEvent(map[string]any{
					"external_sale_id":     "sale-support-001",
					"customer_mobile_e164": "+5511999777777",
					"customer_email":       "support@example.com",
					"paid_at":              paidAt,
				}),
			},
			setup: func(useCase *consumerMocks.HandlePaidWithoutTokenUseCase) {
				useCase.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.HandlePaidWithoutTokenInput) bool {
					return in.ExternalSaleID == "sale-support-001"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			useCase := consumerMocks.NewHandlePaidWithoutTokenUseCase(s.T())
			scenario.setup(useCase)
			consumer := consumers.NewPaidWithoutTokenConsumer(useCase, noop.NewProvider())
			event := any(scenario.args.event)
			if scenario.name == "deve retornar erro para payload inesperado" {
				event = &subscriptionPaidBadPayloadEvent{}
			}
			scenario.expect(consumer.Handle(s.ctx, event.(interface {
				GetEventType() string
				GetPayload() any
			})))
		})
	}
}
