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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	eventsmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SubscriptionEventProjectorSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSubscriptionEventProjector(t *testing.T) {
	suite.Run(t, new(SubscriptionEventProjectorSuite))
}

func (s *SubscriptionEventProjectorSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubscriptionEventProjectorSuite) newEvent(eventType string, payload any) events.Event {
	rawPayload, err := json.Marshal(payload)
	s.Require().NoError(err)

	event := eventsmocks.NewEvent(s.T())
	event.EXPECT().GetEventType().Return(eventType)
	event.EXPECT().GetPayload().Return(outbox.Envelope{
		ID:        "event-id",
		EventType: eventType,
		Payload:   rawPayload,
	}).Once()
	return event
}

func (s *SubscriptionEventProjectorSuite) TestHandle() {
	type args struct {
		event events.Event
	}

	type dependencies struct {
		useCase *consumermocks.MockProjectSubscriptionEventUseCase
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "deve encaminhar envelope para o use case",
			args: args{
				event: s.newEvent("billing.subscription.activated", map[string]any{
					"subscription_id": "sub-123",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.ProjectSubscriptionEvent{
						EventType: "billing.subscription.activated",
						Payload:   json.RawMessage(`{"subscription_id":"sub-123"}`),
					},
				).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro do use case",
			args: args{
				event: s.newEvent("billing.subscription.renewed", map[string]any{
					"subscription_id": "sub-456",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.ProjectSubscriptionEvent{
						EventType: "billing.subscription.renewed",
						Payload:   json.RawMessage(`{"subscription_id":"sub-456"}`),
					},
				).Return(context.DeadlineExceeded).Once()
			},
			expect: func(err error) {
				s.Require().ErrorIs(err, context.DeadlineExceeded)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				useCase: consumermocks.NewMockProjectSubscriptionEventUseCase(s.T()),
			}
			scenario.setup(deps)

			sut := consumers.NewSubscriptionEventProjector(deps.useCase, noop.NewProvider())
			err := sut.Handle(s.ctx, scenario.args.event)

			scenario.expect(err)
		})
	}
}
