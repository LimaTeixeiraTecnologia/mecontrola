package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type fakePendingExpenseGateway struct {
	draft   pendingexpense.Draft
	found   bool
	err     error
	saved   int
	cleared int
}

func (f *fakePendingExpenseGateway) Load(_ context.Context, _ uuid.UUID, _ string) (pendingexpense.Draft, bool, error) {
	return f.draft, f.found, f.err
}

func (f *fakePendingExpenseGateway) Save(_ context.Context, _ uuid.UUID, _ string, _ pendingexpense.Draft) error {
	f.saved++
	return nil
}

func (f *fakePendingExpenseGateway) Clear(_ context.Context, _ uuid.UUID, _ string) error {
	f.cleared++
	return nil
}

type fakeOnboardingTurnRunner struct {
	handled bool
	reply   string
	calls   int
}

func (f *fakeOnboardingTurnRunner) Run(_ context.Context, _ uuid.UUID, _, _ string) (services.OnboardingTurnResult, error) {
	f.calls++
	return services.OnboardingTurnResult{Handled: f.handled, Reply: f.reply}, nil
}

type fakeConfirmExpenseRecorder struct {
	result tools.ExpenseRecorderResult
	err    error
	calls  int
}

func (f *fakeConfirmExpenseRecorder) Execute(_ context.Context, _ tools.ExpenseRecorderInput) (tools.ExpenseRecorderResult, error) {
	f.calls++
	return f.result, f.err
}

type PendingExpensePrioritySuite struct {
	suite.Suite
	wa       *fakeWhatsAppGateway
	fallback *fakeFallback
	parser   *fakeParser
}

func TestPendingExpensePrioritySuite(t *testing.T) {
	suite.Run(t, new(PendingExpensePrioritySuite))
}

func (s *PendingExpensePrioritySuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.fallback = &fakeFallback{reply: "fallback"}
	s.parser = &fakeParser{}
}

func (s *PendingExpensePrioritySuite) TestPendingExpenseConfirmedBeforeOnboarding() {
	pending := &fakePendingExpenseGateway{
		found: true,
		draft: pendingexpense.Draft{
			AmountCents:   13150,
			Merchant:      "farmácia",
			PaymentMethod: "pix",
			Direction:     "outcome",
			CategoryID:    "expense.custo_fixo.medicamentos",
			CategoryPath:  "Custo Fixo > Medicamentos e Farmácia",
		},
	}
	onboarding := &fakeOnboardingTurnRunner{handled: true, reply: "onboarding handling sim"}
	expense := &fakeConfirmExpenseRecorder{
		result: tools.ExpenseRecorderResult{
			Persisted:    true,
			AmountCents:  13150,
			CategoryPath: "Custo Fixo > Medicamentos e Farmácia",
		},
	}

	confidence, err := valueobjects.NewConfidence(1.0)
	require.NoError(s.T(), err)
	s.parser = &fakeParser{intent: intent.Intent{}, confidence: confidence.Value()}

	deps := services.IntentRouterDeps{
		Parser:                     s.parser,
		Fallback:                   s.fallback,
		WhatsAppGateway:            s.wa,
		PendingExpenseConfirmation: pending,
		OnboardingRunner:           onboarding,
		ExpenseRecorder:            expense,
		Location:                   time.UTC,
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)

	result := router.RouteWhatsApp(
		context.Background(),
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "sim", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(1, expense.calls, "expense recorder deve ser chamado")
	s.Equal(0, onboarding.calls, "onboarding NAO deve ser chamado quando ha pending expense confirmado")
}

func (s *PendingExpensePrioritySuite) TestPendingExpenseConfirmedWithLongerText() {
	pending := &fakePendingExpenseGateway{
		found: true,
		draft: pendingexpense.Draft{
			AmountCents:   13150,
			Merchant:      "farmácia",
			PaymentMethod: "pix",
			Direction:     "outcome",
			CategoryID:    "expense.custo_fixo.medicamentos",
			CategoryPath:  "Custo Fixo > Medicamentos e Farmácia",
		},
	}
	expense := &fakeConfirmExpenseRecorder{
		result: tools.ExpenseRecorderResult{Persisted: true, AmountCents: 13150, CategoryPath: "Custo Fixo > Medicamentos e Farmácia"},
	}
	onboarding := &fakeOnboardingTurnRunner{handled: true, reply: "onboarding handling"}

	confidence, err := valueobjects.NewConfidence(1.0)
	require.NoError(s.T(), err)
	s.parser = &fakeParser{intent: intent.Intent{}, confidence: confidence.Value()}

	deps := services.IntentRouterDeps{
		Parser:                     s.parser,
		Fallback:                   s.fallback,
		WhatsAppGateway:            s.wa,
		PendingExpenseConfirmation: pending,
		OnboardingRunner:           onboarding,
		ExpenseRecorder:            expense,
		Location:                   time.UTC,
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)

	result := router.RouteWhatsApp(
		context.Background(),
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "sim, registrar com essa categoria", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(1, expense.calls, "expense recorder deve ser chamado")
	s.Equal(0, onboarding.calls, "onboarding NAO deve ser chamado")
}

func (s *PendingExpensePrioritySuite) TestNoPendingExpense_OnboardingHandles() {
	pending := &fakePendingExpenseGateway{found: false}
	onboarding := &fakeOnboardingTurnRunner{handled: true, reply: "resposta do onboarding"}

	confidence, err := valueobjects.NewConfidence(1.0)
	require.NoError(s.T(), err)
	s.parser = &fakeParser{intent: intent.Intent{}, confidence: confidence.Value()}

	deps := services.IntentRouterDeps{
		Parser:                     s.parser,
		Fallback:                   s.fallback,
		WhatsAppGateway:            s.wa,
		PendingExpenseConfirmation: pending,
		OnboardingRunner:           onboarding,
		Location:                   time.UTC,
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)

	result := router.RouteWhatsApp(
		context.Background(),
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "sim", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, onboarding.calls, "onboarding deve ser chamado quando nao ha pending expense")
	s.Equal("resposta do onboarding", result.Reply)
}
