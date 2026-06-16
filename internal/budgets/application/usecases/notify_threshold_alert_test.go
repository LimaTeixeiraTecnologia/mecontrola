package usecases_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type fakeNotifyManager struct{}

func (fakeNotifyManager) Driver() database.Driver              { return "" }
func (fakeNotifyManager) DBTX(_ context.Context) database.DBTX { return nil }
func (fakeNotifyManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, errors.New("not implemented")
}
func (fakeNotifyManager) Ping(_ context.Context) error     { return nil }
func (fakeNotifyManager) Shutdown(_ context.Context) error { return nil }

type stubChannelGateway struct {
	sentChannel  string
	sentExternal string
	sentText     string
	err          error
}

func (s *stubChannelGateway) SendText(_ context.Context, channel, externalID, text string) error {
	s.sentChannel = channel
	s.sentExternal = externalID
	s.sentText = text
	return s.err
}

func (s *stubChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", errors.New("not used in notify suite")
}

type NotifyThresholdAlertSuite struct {
	suite.Suite
	repo     *mocks.ThresholdAlertSentRepository
	factory  *mocks.RepositoryFactory
	resolver *mocks.UserChannelResolver
}

func TestNotifyThresholdAlert(t *testing.T) {
	suite.Run(t, new(NotifyThresholdAlertSuite))
}

func (s *NotifyThresholdAlertSuite) SetupTest() {
	s.repo = mocks.NewThresholdAlertSentRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.resolver = mocks.NewUserChannelResolver(s.T())
	s.factory.EXPECT().ThresholdAlertSentRepository(mock.Anything).Return(s.repo).Maybe()
}

func (s *NotifyThresholdAlertSuite) TestExecute() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	kind := services.ThresholdAlertCategory

	scenarios := []struct {
		name          string
		setup         func(gw *stubChannelGateway)
		expectOutcome string
		expectError   bool
		expectChannel string
		expectTextHas string
	}{
		{
			name: "envia notificacao com sucesso",
			setup: func(_ *stubChannelGateway) {
				s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
				s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990000"}, true, nil).Once()
				s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()
			},
			expectOutcome: usecases.NotifyOutcomeSent,
			expectChannel: "whatsapp",
			expectTextHas: "utilizou",
		},
		{
			name: "skipa se ja notificado",
			setup: func(_ *stubChannelGateway) {
				s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(true, nil).Once()
			},
			expectOutcome: usecases.NotifyOutcomeAlreadySent,
		},
		{
			name: "outcome=no_channel quando resolver nao retorna preferencia",
			setup: func(_ *stubChannelGateway) {
				s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
				s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{}, false, nil).Once()
			},
			expectOutcome: usecases.NotifyOutcomeNoChannel,
		},
		{
			name: "outcome=channel_failed quando gateway falha",
			setup: func(gw *stubChannelGateway) {
				gw.err = errors.New("meta down")
				s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
				s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511"}, true, nil).Once()
				s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()
			},
			expectOutcome: usecases.NotifyOutcomeChannelFailed,
			expectError:   true,
			expectChannel: "whatsapp",
		},
		{
			name: "outcome=already_sent quando MarkNotified nao atualiza linha (concorrencia)",
			setup: func(_ *stubChannelGateway) {
				s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
				s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{Channel: "telegram", ExternalID: "1234"}, true, nil).Once()
				s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "telegram", mock.Anything).Return(false, nil).Once()
			},
			expectOutcome: usecases.NotifyOutcomeAlreadySent,
			expectChannel: "telegram",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			gw := &stubChannelGateway{}
			scenario.setup(gw)
			uc := usecases.NewNotifyThresholdAlert(fakeNotifyManager{}, s.factory, s.resolver, gw, noop.NewProvider())
			result, err := uc.Execute(context.Background(), usecases.NotifyThresholdAlertInput{
				UserID:               userID,
				BudgetID:             budgetID,
				Kind:                 kind,
				RootSlug:             "alimentacao",
				PercentUsedBps:       8500,
				AmountRemainingCents: 12345,
				RefDay:               refDay,
			})
			if scenario.expectError {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
			s.Equal(scenario.expectOutcome, result.Outcome)
			if scenario.expectChannel != "" {
				s.Equal(scenario.expectChannel, result.Channel)
			}
			if scenario.expectTextHas != "" {
				s.True(strings.Contains(gw.sentText, scenario.expectTextHas), "expected message contains %q, got %q", scenario.expectTextHas, gw.sentText)
			}
		})
	}
}
