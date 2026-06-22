package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
)

type SendSubscriptionNotificationSuite struct {
	suite.Suite
	ctx        context.Context
	obs        *fake.Provider
	senderMock *mocks.NotificationSender
}

func TestSendSubscriptionNotification(t *testing.T) {
	suite.Run(t, new(SendSubscriptionNotificationSuite))
}

func (s *SendSubscriptionNotificationSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.senderMock = mocks.NewNotificationSender(s.T())
}

func (s *SendSubscriptionNotificationSuite) validPayload(subscriptionID string) json.RawMessage {
	b, err := json.Marshal(map[string]string{"subscription_id": subscriptionID})
	s.Require().NoError(err)
	return b
}

type notifDeps struct {
	senderMock *mocks.NotificationSender
}

func (s *SendSubscriptionNotificationSuite) TestExecute() {
	errSentinel := errors.New("sender unavailable")

	scenarios := []struct {
		name         string
		in           input.SendSubscriptionNotificationInput
		dependencies notifDeps
		assert       func(err error)
	}{
		{
			name: "deve retornar nil e chamar sender uma vez no happy path",
			in: input.SendSubscriptionNotificationInput{
				EventType: "subscription_activated",
				Payload:   s.validPayload("sub-001"),
			},
			dependencies: func() notifDeps {
				s.senderMock.EXPECT().
					NotifyTransition(mock.Anything, interfaces.NotificationPayload{
						SubscriptionID: "sub-001",
						EventType:      "subscription_activated",
					}).
					Return(nil).
					Once()
				return notifDeps{senderMock: s.senderMock}
			}(),
			assert: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "falha no sender e absorvida (fire-and-forget): use case retorna nil",
			in: input.SendSubscriptionNotificationInput{
				EventType: "subscription_activated",
				Payload:   s.validPayload("sub-002"),
			},
			dependencies: func() notifDeps {
				s.senderMock.EXPECT().
					NotifyTransition(mock.Anything, interfaces.NotificationPayload{
						SubscriptionID: "sub-002",
						EventType:      "subscription_activated",
					}).
					Return(errSentinel).
					Once()
				return notifDeps{senderMock: s.senderMock}
			}(),
			assert: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar nil sem chamar sender quando payload for invalido",
			in: input.SendSubscriptionNotificationInput{
				EventType: "subscription_activated",
				Payload:   json.RawMessage(`not-valid-json`),
			},
			dependencies: notifDeps{senderMock: s.senderMock},
			assert: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewSendSubscriptionNotification(scenario.dependencies.senderMock, s.obs)
			err := sut.Execute(s.ctx, scenario.in)
			scenario.assert(err)
		})
	}
}
