package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

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
	obs      observability.Observability
	repo     *mocks.ThresholdAlertSentRepository
	resolver *mocks.UserChannelResolver
}

func TestNotifyThresholdAlert(t *testing.T) {
	suite.Run(t, new(NotifyThresholdAlertSuite))
}

func (s *NotifyThresholdAlertSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.repo = mocks.NewThresholdAlertSentRepository(s.T())
	s.resolver = mocks.NewUserChannelResolver(s.T())
}

func (s *NotifyThresholdAlertSuite) TestExecute() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	kind := services.ThresholdAlertCategory

	type dependencies struct {
		gw *stubChannelGateway
	}

	scenarios := []struct {
		name          string
		dependencies  dependencies
		expectOutcome string
		expectError   bool
		expectChannel string
		expectTextHas string
	}{
		{
			name: "envia notificacao com sucesso",
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990000"}, true, nil).Once()
					s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()
					return &stubChannelGateway{}
				}(),
			},
			expectOutcome: NotifyOutcomeSent,
			expectChannel: "whatsapp",
			expectTextHas: "utilizou",
		},
		{
			name: "skipa se ja notificado",
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(true, nil).Once()
					return &stubChannelGateway{}
				}(),
			},
			expectOutcome: NotifyOutcomeAlreadySent,
		},
		{
			name: "outcome=no_channel quando resolver nao retorna preferencia",
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{}, false, nil).Once()
					return &stubChannelGateway{}
				}(),
			},
			expectOutcome: NotifyOutcomeNoChannel,
		},
		{
			name: "outcome=channel_failed quando gateway falha",
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511"}, true, nil).Once()
					s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()
					return &stubChannelGateway{err: errors.New("meta down")}
				}(),
			},
			expectOutcome: NotifyOutcomeChannelFailed,
			expectError:   true,
			expectChannel: "whatsapp",
		},
		{
			name: "outcome=already_sent quando MarkNotified nao atualiza linha (concorrencia)",
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{Channel: "telegram", ExternalID: "1234"}, true, nil).Once()
					s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "telegram", mock.Anything).Return(false, nil).Once()
					return &stubChannelGateway{}
				}(),
			},
			expectOutcome: NotifyOutcomeAlreadySent,
			expectChannel: "telegram",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewNotifyThresholdAlert(s.repo, s.resolver, scenario.dependencies.gw, s.obs)
			result, err := uc.Execute(context.Background(), NotifyThresholdAlertInput{
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
				s.True(strings.Contains(scenario.dependencies.gw.sentText, scenario.expectTextHas), "expected message contains %q, got %q", scenario.expectTextHas, scenario.dependencies.gw.sentText)
			}
		})
	}
}
