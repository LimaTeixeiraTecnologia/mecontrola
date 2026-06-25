package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func buildDestructiveConfirmDefWithTTL(ttl time.Duration) platform.Definition[confirmation.ConfirmState] {
	targets := map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			return st, nil
		},
	}
	executors := map[confirmation.OperationKind]steps.DestructiveExecutor{
		confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.Outcome = int(tools.OutcomeRouted)
			st.Reply = "apagado"
			return st, nil
		},
	}
	return agentwf.NewDestructiveConfirmDefinition(agentwf.DestructiveConfirmDeps{
		Authorize: func(_ context.Context, _ confirmation.ConfirmState) bool { return true },
		Replay:    func(_ context.Context, _ confirmation.ConfirmState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
			return steps.ConfirmAuditBeginResult{}
		},
		OnSettle:       nil,
		Targets:        targets,
		Executors:      executors,
		TTL:            ttl,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})
}

type stubOnboardingRunner struct {
	handled bool
	reply   string
	err     error
	calls   int
}

func (r *stubOnboardingRunner) Run(_ context.Context, _ uuid.UUID, _, _ string) (services.OnboardingTurnResult, error) {
	r.calls++
	return services.OnboardingTurnResult{Handled: r.handled, Reply: r.reply}, r.err
}

type HITLRoutingSuite struct {
	suite.Suite
	ctx context.Context
	wa  *fakeWhatsAppGateway
}

func TestHITLRoutingSuite(t *testing.T) {
	suite.Run(t, new(HITLRoutingSuite))
}

func (s *HITLRoutingSuite) SetupTest() {
	s.ctx = context.Background()
	s.wa = &fakeWhatsAppGateway{}
}

func buildDestructiveConfirmDef(
	targetReply func(op confirmation.OperationKind) string,
	executeOK bool,
) platform.Definition[confirmation.ConfirmState] {
	targets := map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			if targetReply != nil {
				st.PromptText = targetReply(st.OperationKind)
			}
			return st, nil
		},
		confirmation.OperationEditLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			return st, nil
		},
		confirmation.OperationDeleteCard: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			return st, nil
		},
		confirmation.OperationBudgetCommit: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			return st, nil
		},
	}
	executors := map[confirmation.OperationKind]steps.DestructiveExecutor{
		confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			if !executeOK {
				return st, nil
			}
			st.Outcome = int(tools.OutcomeRouted)
			st.Reply = "✅ Último lançamento apagado com sucesso."
			return st, nil
		},
		confirmation.OperationEditLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.Outcome = int(tools.OutcomeRouted)
			st.Reply = "✅ Último lançamento atualizado com sucesso."
			return st, nil
		},
		confirmation.OperationDeleteCard: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.Outcome = int(tools.OutcomeRouted)
			st.Reply = "✅ Cartão removido com sucesso."
			return st, nil
		},
		confirmation.OperationBudgetCommit: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.Outcome = int(tools.OutcomeRouted)
			st.Reply = "✅ Orçamento configurado e ativado com sucesso."
			return st, nil
		},
	}
	return agentwf.NewDestructiveConfirmDefinition(agentwf.DestructiveConfirmDeps{
		Authorize: func(_ context.Context, _ confirmation.ConfirmState) bool { return true },
		Replay:    func(_ context.Context, _ confirmation.ConfirmState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
			return steps.ConfirmAuditBeginResult{}
		},
		OnSettle:       nil,
		Targets:        targets,
		Executors:      executors,
		TTL:            10 * time.Minute,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})
}

func (s *HITLRoutingSuite) buildRouter(intentVal intent.Intent, confirmDef platform.Definition[confirmation.ConfirmState]) *services.IntentRouter {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: intentVal},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)
	return router
}

func (s *HITLRoutingSuite) TestDestructiveKind_FirstMessage_SuspendsWaitingConfirmation() {
	type scenario struct {
		name     string
		kind     intent.Kind
		mkIntent func() intent.Intent
	}
	scenarios := []scenario{
		{
			name:     "KindDeleteLastTransaction suspende na 1a mensagem",
			kind:     intent.KindDeleteLastTransaction,
			mkIntent: func() intent.Intent { return intent.NewDeleteLastTransaction() },
		},
		{
			name: "KindEditLastTransaction suspende na 1a mensagem",
			kind: intent.KindEditLastTransaction,
			mkIntent: func() intent.Intent {
				in, err := intent.NewEditLastTransaction(500)
				s.Require().NoError(err)
				return in
			},
		},
		{
			name: "KindDeleteCard suspende na 1a mensagem",
			kind: intent.KindDeleteCard,
			mkIntent: func() intent.Intent {
				in, err := intent.NewDeleteCard("Nubank")
				s.Require().NoError(err)
				return in
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.wa = &fakeWhatsAppGateway{}
			confirmDef := buildDestructiveConfirmDef(nil, false)
			router := s.buildRouter(sc.mkIntent(), confirmDef)

			result := router.RouteWhatsApp(
				s.ctx,
				services.Principal{UserID: uuid.New()},
				services.InboundMessage{Text: "quero apagar", WhatsAppTo: "+5511999"},
			)

			s.Equal(tools.OutcomeClarify, result.Outcome, sc.name)
			s.NotEmpty(result.Reply, sc.name)
			s.Len(s.wa.sent, 1, sc.name)
		})
	}
}

func (s *HITLRoutingSuite) TestDestructiveKind_SecondMessage_Confirm_Executes() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDef(nil, true)

	userID := uuid.New()
	peer := "+5511999"

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: intent.NewDeleteLastTransaction()},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	first := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "apagar último lançamento", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeClarify, first.Outcome)

	second := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "sim", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeRouted, second.Outcome)
	s.Contains(second.Reply, "apagado")
}

func (s *HITLRoutingSuite) TestDestructiveKind_SecondMessage_Cancel_DoesNotExecute() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDef(nil, true)

	userID := uuid.New()
	peer := "+5511999"

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: intent.NewDeleteLastTransaction()},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	first := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "apagar último lançamento", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeClarify, first.Outcome)

	second := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "não", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeRouted, second.Outcome)
	s.Contains(second.Reply, "cancelad")
}

func (s *HITLRoutingSuite) TestNonDestructiveKind_NoGate() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDef(nil, true)

	expenseIntent, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  2000,
		Merchant:     "padaria",
		CategoryHint: "alimentação",
	})
	s.Require().NoError(err)

	expenseLogger := &fakeExpenseLogger{
		result: tools.ExpenseRecorderResult{Persisted: true, AmountCents: 2000, CategoryPath: "Alimentação"},
	}

	kernel := newKernelWithExpenseRecorder(expenseLogger, nil)
	kernel.ConfirmEngine = confirmEngine
	kernel.ConfirmDef = confirmDef

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: expenseIntent},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		ExpenseRecorder: expenseLogger,
		Kernel:          kernel,
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	result := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "gastei 20 na padaria", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, expenseLogger.calls, "expense recorder deve ser chamado via kernel sem gate destrutivo")
}

func (s *HITLRoutingSuite) TestDestructiveKind_Expired_FallThrough() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDefWithTTL(time.Millisecond)

	deleteIntent := intent.NewDeleteLastTransaction()

	expenseLogger := &fakeExpenseLogger{
		result: tools.ExpenseRecorderResult{Persisted: true, AmountCents: 1000, CategoryPath: "Alimentação"},
	}

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: deleteIntent},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		ExpenseRecorder: expenseLogger,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	principal := services.Principal{UserID: uuid.New()}
	inbound := services.InboundMessage{Text: "apagar último lançamento", WhatsAppTo: "+5511999"}

	result1 := router.RouteWhatsApp(s.ctx, principal, inbound)
	s.Equal(tools.OutcomeClarify, result1.Outcome, "1ª mensagem deve suspender aguardando confirmação")

	time.Sleep(5 * time.Millisecond)

	expenseIntent, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  500,
		Merchant:     "padaria",
		CategoryHint: "alimentação",
	})
	s.Require().NoError(err)

	deps2 := deps
	deps2.Parser = &fakeParser{intent: expenseIntent}
	router2, err := services.NewIntentRouter(obs, deps2)
	s.Require().NoError(err)

	result2 := router2.RouteWhatsApp(s.ctx, principal, services.InboundMessage{Text: "gastei 5 na padaria", WhatsAppTo: "+5511999"})
	s.NotEqual(tools.OutcomeClarify, result2.Outcome, "após expiração, mensagem deve cair no parse (fall-through), não ficar presa no HITL")
}

func (s *HITLRoutingSuite) TestDestructiveKind_EditLast_PassesNewAmount() {
	obs := fake.NewProvider()
	store := newE2EStore()

	var capturedState confirmation.ConfirmState
	confirmDef := agentwf.NewDestructiveConfirmDefinition(agentwf.DestructiveConfirmDeps{
		Authorize: func(_ context.Context, _ confirmation.ConfirmState) bool { return true },
		Replay:    func(_ context.Context, _ confirmation.ConfirmState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
			return steps.ConfirmAuditBeginResult{}
		},
		OnSettle: nil,
		Targets: map[confirmation.OperationKind]steps.TargetResolver{
			confirmation.OperationEditLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
				capturedState = st
				st.PromptText = "confirme?"
				return st, nil
			},
		},
		Executors:      map[confirmation.OperationKind]steps.DestructiveExecutor{},
		TTL:            10 * time.Minute,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})

	editIntent, err := intent.NewEditLastTransaction(7777)
	s.Require().NoError(err)

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: editIntent},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: platform.NewEngine[confirmation.ConfirmState](store, obs),
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "editar último para 77,77", WhatsAppTo: "+5511999"},
	)

	s.Equal(confirmation.OperationEditLast, capturedState.OperationKind)
	s.Equal(int64(7777), capturedState.NewAmountCents)
}

func (s *HITLRoutingSuite) TestDestructiveKind_DeleteCard_PassesCardName() {
	obs := fake.NewProvider()
	store := newE2EStore()

	var capturedState confirmation.ConfirmState
	confirmDef := agentwf.NewDestructiveConfirmDefinition(agentwf.DestructiveConfirmDeps{
		Authorize: func(_ context.Context, _ confirmation.ConfirmState) bool { return true },
		Replay:    func(_ context.Context, _ confirmation.ConfirmState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
			return steps.ConfirmAuditBeginResult{}
		},
		OnSettle: nil,
		Targets: map[confirmation.OperationKind]steps.TargetResolver{
			confirmation.OperationDeleteCard: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
				capturedState = st
				st.PromptText = "confirme?"
				return st, nil
			},
		},
		Executors:      map[confirmation.OperationKind]steps.DestructiveExecutor{},
		TTL:            10 * time.Minute,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})

	deleteCardIntent, err := intent.NewDeleteCard("Nubank")
	s.Require().NoError(err)

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: deleteCardIntent},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: platform.NewEngine[confirmation.ConfirmState](store, obs),
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "apagar cartão Nubank", WhatsAppTo: "+5511999"},
	)

	s.Equal(confirmation.OperationDeleteCard, capturedState.OperationKind)
	s.Equal("Nubank", capturedState.CardName)
}

func (s *HITLRoutingSuite) TestHITLResumeWinsWithPassiveOnboardingRunner() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDef(nil, true)
	onboarding := &stubOnboardingRunner{handled: false}

	userID := uuid.New()
	peer := "+5511999"

	deps := services.IntentRouterDeps{
		Parser:           &fakeParser{intent: intent.NewDeleteLastTransaction()},
		OnboardingRunner: onboarding,
		Fallback:         &fakeFallback{reply: "fallback"},
		WhatsAppGateway:  s.wa,
		Location:         time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	first := router.RouteWhatsApp(s.ctx, services.Principal{UserID: userID},
		services.InboundMessage{Text: "apagar último lançamento", WhatsAppTo: peer})
	s.Equal(tools.OutcomeClarify, first.Outcome)

	second := router.RouteWhatsApp(s.ctx, services.Principal{UserID: userID},
		services.InboundMessage{Text: "sim", WhatsAppTo: peer})
	s.Equal(tools.OutcomeRouted, second.Outcome, "onboarding passivo não pode engolir o resume do gate HITL")
	s.Contains(second.Reply, "apagado")
	s.Equal(1, onboarding.calls, "resume do HITL intercepta antes do onboarding; onboarding só é consultado na 1ª mensagem")
}

func (s *HITLRoutingSuite) TestActiveOnboardingTakesPriorityOverDestructiveGate() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDef(nil, true)
	onboarding := &stubOnboardingRunner{handled: true, reply: "vamos terminar seu cadastro primeiro"}

	deps := services.IntentRouterDeps{
		Parser:           &fakeParser{intent: intent.NewDeleteLastTransaction()},
		OnboardingRunner: onboarding,
		Fallback:         &fakeFallback{reply: "fallback"},
		WhatsAppGateway:  s.wa,
		Location:         time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	result := router.RouteWhatsApp(s.ctx, services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "apagar último lançamento", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindConfigureBudget, result.Kind, "onboarding ativo deve responder antes de qualquer gate destrutivo")
	s.Equal("vamos terminar seu cadastro primeiro", result.Reply)
	s.Equal(1, onboarding.calls)
}

func (s *HITLRoutingSuite) TestDestructiveKind_AmbiguousRepromptCycle() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildDestructiveConfirmDef(nil, true)

	deleteIntent := intent.NewDeleteLastTransaction()

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: deleteIntent},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	principal := services.Principal{UserID: uuid.New()}

	result1 := router.RouteWhatsApp(s.ctx, principal, services.InboundMessage{Text: "apagar último lançamento", WhatsAppTo: "+5511999"})
	s.Equal(tools.OutcomeClarify, result1.Outcome, "1ª mensagem deve suspender aguardando confirmação")

	result2 := router.RouteWhatsApp(s.ctx, principal, services.InboundMessage{Text: "talvez", WhatsAppTo: "+5511999"})
	s.Equal(tools.OutcomeClarify, result2.Outcome, "texto ambíguo 1ª vez deve re-perguntar (reprompt)")

	result3 := router.RouteWhatsApp(s.ctx, principal, services.InboundMessage{Text: "talvez", WhatsAppTo: "+5511999"})
	s.Equal(tools.OutcomeRouted, result3.Outcome, "texto ambíguo 2ª vez deve cancelar")
	s.NotEmpty(result3.Reply, "deve haver mensagem de cancelamento")
}
