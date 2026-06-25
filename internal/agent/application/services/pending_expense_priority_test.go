package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

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

func (s *PendingExpensePrioritySuite) buildRouterWithKernelAndOnboarding(
	store platform.Store,
	resolver steps.CategoryResolverFunc,
	persistFn steps.PersistFunc,
	onboarding *fakeOnboardingTurnRunner,
	expense tools.ExpenseRecorder,
) *services.IntentRouter {
	obs := fake.NewProvider()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	settleReg := services.NewSettleRegistry()

	confidence, err := valueobjects.NewConfidence(1.0)
	require.NoError(s.T(), err)
	s.parser = &fakeParser{intent: mustBuildExpenseIntent2(13150, "farmácia", "medicamentos"), confidence: confidence.Value()}

	deps := services.IntentRouterDeps{
		Parser:           s.parser,
		Fallback:         s.fallback,
		WhatsAppGateway:  s.wa,
		OnboardingRunner: onboarding,
		ExpenseRecorder:  expense,
		Location:         time.UTC,
		Kernel: &services.KernelDeps{
			Engine:           engine,
			SettleReg:        settleReg,
			CategoryResolver: resolver,
			PersistFn:        persistFn,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	require.NoError(s.T(), err)
	return router
}

func (s *PendingExpensePrioritySuite) TestKernelResumePriorBeforeOnboarding() {
	store := newE2EStore()
	obs := fake.NewProvider()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)

	candidates := []string{"Custo Fixo > Medicamentos", "Saúde > Farmácia"}
	resolver := func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
		if st.ForceCategory == nil && st.AwaitingKind == "" {
			return st, &tools.CategoryAmbiguousError{Hint: "medicamentos", Candidates: candidates}
		}
		cat := candidates[0]
		if st.ForceCategory != nil {
			cat = *st.ForceCategory
		}
		st.CategoryID = cat
		st.CategoryPath = cat
		return st, nil
	}
	expense := &fakeConfirmExpenseRecorder{
		result: tools.ExpenseRecorderResult{Persisted: true, AmountCents: 13150, CategoryPath: candidates[0]},
	}
	onboarding := &fakeOnboardingTurnRunner{handled: true, reply: "onboarding reply"}

	def := agentwf.NewTransactionsWriteDefinition(agentwf.TransactionsWriteDeps{
		Authorize: func(_ context.Context, _ steps.ExpenseState) bool { return true },
		Replay:    func(_ context.Context, _ steps.ExpenseState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
			return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
		},
		OnSettle: nil,
		Resolver: resolver,
		Persist: func(_ context.Context, st steps.ExpenseState) (steps.PersistResult, error) {
			return steps.PersistResult{AmountCents: 13150, CategoryPath: candidates[0]}, nil
		},
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})

	userID := uuid.New()
	correlationKey := userID.String() + ":whatsapp"
	initial := steps.ExpenseState{
		UserID:          userID,
		Channel:         "whatsapp",
		AmountCents:     13150,
		Merchant:        "farmácia",
		PaymentMethod:   "pix",
		Direction:       "outcome",
		TransactionKind: pendingexpense.TransactionKindExpense,
		Kind:            intent.KindRecordExpense,
	}
	result1, err := engine.Start(context.Background(), def, correlationKey, initial)
	require.NoError(s.T(), err)
	require.Equal(s.T(), platform.RunStatusSuspended, result1.Status)

	router := s.buildRouterWithKernelAndOnboarding(store, resolver, func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
		return steps.PersistResult{AmountCents: 13150, CategoryPath: candidates[0]}, nil
	}, onboarding, expense)

	result := router.RouteWhatsApp(
		context.Background(),
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "1", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(0, onboarding.calls, "onboarding NAO deve ser chamado quando ha kernel run suspenso")
}

func (s *PendingExpensePrioritySuite) TestNoPendingKernelRun_OnboardingHandles() {
	store := newE2EStore()
	onboarding := &fakeOnboardingTurnRunner{handled: true, reply: "resposta do onboarding"}
	expense := &fakeConfirmExpenseRecorder{}

	confidence, err := valueobjects.NewConfidence(1.0)
	require.NoError(s.T(), err)
	s.parser = &fakeParser{intent: intent.Intent{}, confidence: confidence.Value()}

	obs := fake.NewProvider()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	settleReg := services.NewSettleRegistry()

	deps := services.IntentRouterDeps{
		Parser:           s.parser,
		Fallback:         s.fallback,
		WhatsAppGateway:  s.wa,
		OnboardingRunner: onboarding,
		ExpenseRecorder:  expense,
		Location:         time.UTC,
		Kernel: &services.KernelDeps{
			Engine:           engine,
			SettleReg:        settleReg,
			CategoryResolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) { return st, nil },
			PersistFn: func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
				return steps.PersistResult{}, nil
			},
		},
	}
	router, routerErr := services.NewIntentRouter(obs, deps)
	require.NoError(s.T(), routerErr)

	result := router.RouteWhatsApp(
		context.Background(),
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "sim", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, onboarding.calls, "onboarding deve ser chamado quando nao ha kernel run suspenso")
	s.Equal("resposta do onboarding", result.Reply)
}
