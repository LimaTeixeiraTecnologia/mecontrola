package consumers_test

import (
	"context"
	"fmt"
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

type AuthEventsConsumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestAuthEventsConsumer(t *testing.T) {
	suite.Run(t, new(AuthEventsConsumerSuite))
}

func (s *AuthEventsConsumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthEventsConsumerSuite) newEnvelopeEvent(eventType string, payload []byte) *eventsmocks.Event {
	evt := eventsmocks.NewEvent(s.T())
	evt.EXPECT().GetEventType().Return(eventType)
	evt.EXPECT().GetPayload().Return(outbox.Envelope{
		ID:        "env-id-" + eventType,
		EventType: eventType,
		Payload:   payload,
	}).Once()
	return evt
}

func (s *AuthEventsConsumerSuite) TestHandle_AuthEvents() {
	type args struct {
		eventType string
		payload   []byte
	}
	type dependencies struct {
		projectAuthEvent        *consumermocks.MockProjectAuthEventUseCase
		anonymizeUserAuthEvents *consumermocks.MockAnonymizeUserAuthEventsUseCase
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "deve chamar projectAuthEvent para auth.principal_established",
			args: args{eventType: "auth.principal_established", payload: []byte(`{"event_id":"x"}`)},
			setup: func(deps dependencies) {
				deps.projectAuthEvent.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.ProjectAuthEvent) bool {
					return in.EventType == "auth.principal_established"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve chamar projectAuthEvent para auth.failed",
			args: args{eventType: "auth.failed", payload: []byte(`{"event_id":"x"}`)},
			setup: func(deps dependencies) {
				deps.projectAuthEvent.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.ProjectAuthEvent) bool {
					return in.EventType == "auth.failed"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve chamar projectAuthEvent para auth.unknown_user",
			args: args{eventType: "auth.unknown_user", payload: []byte(`{"event_id":"x"}`)},
			setup: func(deps dependencies) {
				deps.projectAuthEvent.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.ProjectAuthEvent) bool {
					return in.EventType == "auth.unknown_user"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro do projectAuthEvent",
			args: args{eventType: "auth.principal_established", payload: []byte(`{}`)},
			setup: func(deps dependencies) {
				deps.projectAuthEvent.EXPECT().Execute(mock.Anything, mock.Anything).
					Return(fmt.Errorf("usecase error")).Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				projectAuthEvent:        consumermocks.NewMockProjectAuthEventUseCase(s.T()),
				anonymizeUserAuthEvents: consumermocks.NewMockAnonymizeUserAuthEventsUseCase(s.T()),
			}
			scenario.setup(deps)

			evt := s.newEnvelopeEvent(scenario.args.eventType, scenario.args.payload)
			sut := consumers.NewAuthEventsConsumer(deps.projectAuthEvent, deps.anonymizeUserAuthEvents, noop.NewProvider())
			err := sut.Handle(s.ctx, evt)
			scenario.expect(err)
		})
	}
}

func (s *AuthEventsConsumerSuite) TestHandle_UserDeleted() {
	type args struct {
		payload []byte
	}
	type dependencies struct {
		projectAuthEvent        *consumermocks.MockProjectAuthEventUseCase
		anonymizeUserAuthEvents *consumermocks.MockAnonymizeUserAuthEventsUseCase
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "deve chamar anonymizeUserAuthEvents para user.deleted",
			args: args{payload: []byte(`{"user_id":"some-id"}`)},
			setup: func(deps dependencies) {
				deps.anonymizeUserAuthEvents.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(in input.AnonymizeUserAuthEvents) bool {
					return len(in.Payload) > 0
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro do anonymizeUserAuthEvents",
			args: args{payload: []byte(`{"user_id":"some-id"}`)},
			setup: func(deps dependencies) {
				deps.anonymizeUserAuthEvents.EXPECT().Execute(mock.Anything, mock.Anything).
					Return(fmt.Errorf("usecase error")).Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				projectAuthEvent:        consumermocks.NewMockProjectAuthEventUseCase(s.T()),
				anonymizeUserAuthEvents: consumermocks.NewMockAnonymizeUserAuthEventsUseCase(s.T()),
			}
			scenario.setup(deps)

			evt := s.newEnvelopeEvent("user.deleted", scenario.args.payload)
			sut := consumers.NewAuthEventsConsumer(deps.projectAuthEvent, deps.anonymizeUserAuthEvents, noop.NewProvider())
			err := sut.Handle(s.ctx, evt)
			scenario.expect(err)
		})
	}
}

func (s *AuthEventsConsumerSuite) TestHandle_UnexpectedPayloadType() {
	s.Run("payload nao-Envelope deve retornar erro", func() {
		evt := eventsmocks.NewEvent(s.T())
		evt.EXPECT().GetPayload().Return("not-an-envelope").Once()

		projectUC := consumermocks.NewMockProjectAuthEventUseCase(s.T())
		anonymizeUC := consumermocks.NewMockAnonymizeUserAuthEventsUseCase(s.T())
		sut := consumers.NewAuthEventsConsumer(projectUC, anonymizeUC, noop.NewProvider())
		err := sut.Handle(s.ctx, evt)

		s.Require().Error(err)
		s.ErrorContains(err, "unexpected payload type")
	})
}

func (s *AuthEventsConsumerSuite) TestHandle_UnknownEventType() {
	s.Run("event type desconhecido deve retornar erro", func() {
		evt := s.newEnvelopeEvent("unknown.event", []byte(`{}`))

		projectUC := consumermocks.NewMockProjectAuthEventUseCase(s.T())
		anonymizeUC := consumermocks.NewMockAnonymizeUserAuthEventsUseCase(s.T())
		sut := consumers.NewAuthEventsConsumer(projectUC, anonymizeUC, noop.NewProvider())
		err := sut.Handle(s.ctx, evt)

		s.Require().Error(err)
		s.ErrorContains(err, "unhandled event type")
	})
}
