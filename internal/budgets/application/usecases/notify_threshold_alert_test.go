package usecases

import (
	"context"
	"errors"
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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

type stubChannelGateway struct {
	sentTemplate notification.TemplateMessage
	templateSent bool
	textSent     bool
	err          error
}

func (s *stubChannelGateway) SendText(_ context.Context, _, _, _ string) error {
	s.textSent = true
	return s.err
}

func (s *stubChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", errors.New("not used in notify suite")
}

func (s *stubChannelGateway) SendTemplate(_ context.Context, message notification.TemplateMessage) (string, error) {
	s.templateSent = true
	s.sentTemplate = message
	if s.err != nil {
		return "", s.err
	}
	return "wamid.notify", nil
}

type stubTemplateCatalog struct {
	template appinterfaces.AlertTemplate
	found    bool
}

func (s stubTemplateCatalog) Lookup(_ services.ThresholdAlertKind) (appinterfaces.AlertTemplate, bool) {
	return s.template, s.found
}

type stubTimezoneResolver struct {
	location *time.Location
	err      error
}

func (s stubTimezoneResolver) Resolve(_ context.Context, _ uuid.UUID) (*time.Location, error) {
	return s.location, s.err
}

type stubConsentReader struct {
	consented bool
	err       error
}

func (s stubConsentReader) HasConsent(_ context.Context, _ uuid.UUID) (bool, error) {
	return s.consented, s.err
}

type stubAlertContextRecorder struct {
	calls    int
	kind     string
	topic    string
	threadID string
	err      error
}

func (s *stubAlertContextRecorder) Record(_ context.Context, _, threadID, kind, followUpTopic string, _ time.Time) error {
	s.calls++
	s.kind = kind
	s.topic = followUpTopic
	s.threadID = threadID
	return s.err
}

type NotifyThresholdAlertSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	repo     *mocks.ThresholdAlertSentRepository
	resolver *mocks.UserChannelResolver
}

func TestNotifyThresholdAlert(t *testing.T) {
	suite.Run(t, new(NotifyThresholdAlertSuite))
}

func (s *NotifyThresholdAlertSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.repo = mocks.NewThresholdAlertSentRepository(s.T())
	s.resolver = mocks.NewUserChannelResolver(s.T())
}

func approvedUtility() stubTemplateCatalog {
	return stubTemplateCatalog{
		found: true,
		template: appinterfaces.AlertTemplate{
			Name:         "mecontrola_category_threshold_80",
			LanguageCode: "pt_BR",
			Category:     services.TemplateCategoryUtility,
			Status:       services.TemplateStatusApproved,
		},
	}
}

func (s *NotifyThresholdAlertSuite) TestExecute() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	kind := services.ThresholdAlertCategory80
	preference := appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990000"}

	type dependencies struct {
		gw        *stubChannelGateway
		catalog   stubTemplateCatalog
		timezones stubTimezoneResolver
		consents  stubConsentReader
		quietFrom time.Duration
		quietTo   time.Duration
	}

	scenarios := []struct {
		name           string
		kind           services.ThresholdAlertKind
		dependencies   dependencies
		expectOutcome  string
		expectReason   string
		expectError    bool
		expectTemplate bool
	}{
		{
			name: "envia por template aprovado e marca notificado depois do sucesso",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
					s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog:   approvedUtility(),
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome:  NotifyOutcomeSent,
			expectTemplate: true,
		},
		{
			name: "template PENDING bloqueia envio real",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog: stubTemplateCatalog{found: true, template: appinterfaces.AlertTemplate{
					Name:         "mecontrola_category_threshold_80",
					LanguageCode: "pt_BR",
					Category:     services.TemplateCategoryUtility,
					Status:       services.TemplateStatusPending,
				}},
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome: NotifyOutcomeSuppressed,
			expectReason:  services.SuppressedTemplateUnapproved.String(),
		},
		{
			name: "template ausente bloqueia envio real",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog:   stubTemplateCatalog{found: false},
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome: NotifyOutcomeSuppressed,
			expectReason:  services.SuppressedTemplateUnapproved.String(),
		},
		{
			name: "MARKETING sem opt-in bloqueia envio",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog: stubTemplateCatalog{found: true, template: appinterfaces.AlertTemplate{
					Name:         "mecontrola_budget_not_reviewed_day_3",
					LanguageCode: "pt_BR",
					Category:     services.TemplateCategoryMarketing,
					Status:       services.TemplateStatusApproved,
				}},
				timezones: stubTimezoneResolver{location: time.UTC},
				consents:  stubConsentReader{consented: false},
			},
			expectOutcome: NotifyOutcomeSuppressed,
			expectReason:  services.SuppressedOptInMissing.String(),
		},
		{
			name: "quiet hours bloqueia envio sem marcar notificado",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog:   approvedUtility(),
				timezones: stubTimezoneResolver{location: time.UTC},
				quietFrom: 0,
				quietTo:   24 * time.Hour,
			},
			expectOutcome: NotifyOutcomeSuppressed,
			expectReason:  services.SuppressedQuietHours.String(),
		},
		{
			name: "falha Meta nao marca notificado",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
					return &stubChannelGateway{err: errors.New("meta down")}
				}(),
				catalog:   approvedUtility(),
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome:  NotifyOutcomeChannelFailed,
			expectError:    true,
			expectTemplate: true,
		},
		{
			name: "kind fora do Release 1 e bloqueado",
			kind: services.ThresholdAlertGoal,
			dependencies: dependencies{
				gw:        &stubChannelGateway{},
				catalog:   approvedUtility(),
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome: NotifyOutcomeSuppressed,
			expectReason:  services.SuppressedKindBlocked.String(),
		},
		{
			name: "ja notificado nao reenvia",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(true, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog:   approvedUtility(),
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome: NotifyOutcomeAlreadySent,
		},
		{
			name: "sem canal preferencial suprime",
			kind: kind,
			dependencies: dependencies{
				gw: func() *stubChannelGateway {
					s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
					s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(appinterfaces.UserChannelPreference{}, false, nil).Once()
					return &stubChannelGateway{}
				}(),
				catalog:   approvedUtility(),
				timezones: stubTimezoneResolver{location: time.UTC},
			},
			expectOutcome: NotifyOutcomeSuppressed,
			expectReason:  services.SuppressedNoChannel.String(),
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewNotifyThresholdAlert(
				s.repo,
				s.resolver,
				scenario.dependencies.gw,
				scenario.dependencies.catalog,
				scenario.dependencies.timezones,
				scenario.dependencies.consents,
				nil,
				scenario.dependencies.quietFrom,
				scenario.dependencies.quietTo,
				s.obs,
			)

			result, err := uc.Execute(s.ctx, NotifyThresholdAlertInput{
				UserID:               userID,
				BudgetID:             budgetID,
				Kind:                 scenario.kind,
				RootSlug:             "alimentacao",
				PercentUsedBps:       8500,
				AmountRemainingCents: 12345,
				PlannedCents:         100000,
				SpentCents:           87655,
				RefDay:               refDay,
			})

			if scenario.expectError {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
			s.Equal(scenario.expectOutcome, result.Outcome)
			if scenario.expectReason != "" {
				s.Equal(scenario.expectReason, result.Reason)
			}
			s.Equal(scenario.expectTemplate, scenario.dependencies.gw.templateSent)
			s.False(scenario.dependencies.gw.textSent, "envio proativo nao pode usar texto livre")
		})
	}
}

func (s *NotifyThresholdAlertSuite) TestExecuteSendsFourParametersForCategoryThreshold() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	kind := services.ThresholdAlertCategory80

	s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
	s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).
		Return(appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990000"}, true, nil).Once()
	s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()

	gw := &stubChannelGateway{}
	uc := NewNotifyThresholdAlert(
		s.repo,
		s.resolver,
		gw,
		approvedUtility(),
		stubTimezoneResolver{location: time.UTC},
		stubConsentReader{},
		nil,
		0,
		0,
		s.obs,
	)

	result, err := uc.Execute(s.ctx, NotifyThresholdAlertInput{
		UserID:               userID,
		BudgetID:             budgetID,
		Kind:                 kind,
		RootSlug:             "alimentacao",
		PercentUsedBps:       8000,
		AmountRemainingCents: 20000,
		PlannedCents:         100000,
		SpentCents:           80000,
		RefDay:               refDay,
	})

	s.Require().NoError(err)
	s.Equal(NotifyOutcomeSent, result.Outcome)
	s.Require().Len(gw.sentTemplate.Components, 1)
	s.Require().Len(gw.sentTemplate.Components[0].Parameters, 4)
	s.Equal("alimentacao", gw.sentTemplate.Components[0].Parameters[0].Text)
	s.Equal("R$1000,00", gw.sentTemplate.Components[0].Parameters[1].Text)
	s.Equal("R$800,00", gw.sentTemplate.Components[0].Parameters[2].Text)
	s.Equal("R$200,00", gw.sentTemplate.Components[0].Parameters[3].Text)
}

func (s *NotifyThresholdAlertSuite) TestExecuteRecordsAlertContextOnlyOnRealSend() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	kind := services.ThresholdAlertCategory80
	preference := appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990000"}

	s.Run("envio bem sucedido grava contexto de follow-up", func() {
		s.SetupTest()
		s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
		s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
		s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()

		recorder := &stubAlertContextRecorder{}
		uc := NewNotifyThresholdAlert(s.repo, s.resolver, &stubChannelGateway{}, approvedUtility(),
			stubTimezoneResolver{location: time.UTC}, stubConsentReader{}, recorder, 0, 0, s.obs)

		result, err := uc.Execute(s.ctx, NotifyThresholdAlertInput{
			UserID: userID, BudgetID: budgetID, Kind: kind, RootSlug: "alimentacao",
			PlannedCents: 100000, SpentCents: 85000, AmountRemainingCents: 15000, RefDay: refDay,
		})

		s.Require().NoError(err)
		s.Equal(NotifyOutcomeSent, result.Outcome)
		s.Equal(1, recorder.calls)
		s.Equal("category_threshold_80", recorder.kind)
		s.Equal(services.AlertFollowUpTopic(kind), recorder.topic)
		s.Equal("+5511999990000", recorder.threadID)
	})

	s.Run("supressao nao grava contexto", func() {
		s.SetupTest()
		s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
		s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()

		recorder := &stubAlertContextRecorder{}
		uc := NewNotifyThresholdAlert(s.repo, s.resolver, &stubChannelGateway{},
			stubTemplateCatalog{found: false},
			stubTimezoneResolver{location: time.UTC}, stubConsentReader{}, recorder, 0, 0, s.obs)

		result, err := uc.Execute(s.ctx, NotifyThresholdAlertInput{
			UserID: userID, BudgetID: budgetID, Kind: kind, RefDay: refDay,
		})

		s.Require().NoError(err)
		s.Equal(NotifyOutcomeSuppressed, result.Outcome)
		s.Zero(recorder.calls)
	})

	s.Run("falha ao gravar contexto nao invalida o envio", func() {
		s.SetupTest()
		s.repo.EXPECT().IsNotified(mock.Anything, userID, budgetID, kind, refDay).Return(false, nil).Once()
		s.resolver.EXPECT().ResolvePreferred(mock.Anything, userID).Return(preference, true, nil).Once()
		s.repo.EXPECT().MarkNotified(mock.Anything, userID, budgetID, kind, refDay, "whatsapp", mock.Anything).Return(true, nil).Once()

		recorder := &stubAlertContextRecorder{err: errors.New("memory down")}
		uc := NewNotifyThresholdAlert(s.repo, s.resolver, &stubChannelGateway{}, approvedUtility(),
			stubTimezoneResolver{location: time.UTC}, stubConsentReader{}, recorder, 0, 0, s.obs)

		result, err := uc.Execute(s.ctx, NotifyThresholdAlertInput{
			UserID: userID, BudgetID: budgetID, Kind: kind, RootSlug: "alimentacao",
			PlannedCents: 100000, SpentCents: 85000, AmountRemainingCents: 15000, RefDay: refDay,
		})

		s.Require().NoError(err)
		s.Equal(NotifyOutcomeSent, result.Outcome)
	})
}
