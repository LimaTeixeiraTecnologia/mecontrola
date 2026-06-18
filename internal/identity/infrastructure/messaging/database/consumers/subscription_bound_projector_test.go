package consumers_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	consumermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers/mocks"
	eventsmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SubscriptionBoundProjectorSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSubscriptionBoundProjector(t *testing.T) {
	suite.Run(t, new(SubscriptionBoundProjectorSuite))
}

func (s *SubscriptionBoundProjectorSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubscriptionBoundProjectorSuite) newEnvelopeEvent(eventType string, payload []byte) *eventsmocks.Event {
	evt := eventsmocks.NewEvent(s.T())
	evt.EXPECT().GetEventType().Return(eventType)
	evt.EXPECT().GetPayload().Return(outbox.Envelope{
		ID:        "env-bound-" + eventType,
		EventType: eventType,
		Payload:   payload,
	}).Once()
	return evt
}

func (s *SubscriptionBoundProjectorSuite) TestHandle() {
	type args struct {
		useEnvelope bool
		eventType   string
		payload     []byte
	}

	type dependencies struct {
		useCase *consumermocks.MockProjectSubscriptionBoundUseCase
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "onboarding.subscription_bound com payload valido chama use case uma vez",
			args: args{
				useEnvelope: true,
				eventType:   "onboarding.subscription_bound",
				payload: func() []byte {
					b, _ := json.Marshal(map[string]any{
						"subscription_id": "sub-bound-123",
						"funnel_token":    "tk-abc",
					})
					return b
				}(),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					mock.MatchedBy(func(in input.ProjectSubscriptionEvent) bool {
						return in.EventType == "onboarding.subscription_bound"
					}),
				).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "payload nao-Envelope retorna erro sem chamar use case",
			args: args{
				useEnvelope: false,
			},
			setup: func(deps dependencies) {},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "unexpected payload type")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				useCase: consumermocks.NewMockProjectSubscriptionBoundUseCase(s.T()),
			}
			scenario.setup(deps)

			sut := consumers.NewSubscriptionBoundProjector(deps.useCase, noop.NewProvider())

			if scenario.args.useEnvelope {
				evt := s.newEnvelopeEvent(scenario.args.eventType, scenario.args.payload)
				err := sut.Handle(s.ctx, evt)
				scenario.expect(err)
			} else {
				badEvt := eventsmocks.NewEvent(s.T())
				badEvt.EXPECT().GetPayload().Return("not-an-envelope")
				err := sut.Handle(s.ctx, badEvt)
				scenario.expect(err)
			}
		})
	}
}
