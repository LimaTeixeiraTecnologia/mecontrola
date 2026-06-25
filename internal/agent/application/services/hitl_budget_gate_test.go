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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type HITLBudgetGateSuite struct {
	suite.Suite
	ctx context.Context
	wa  *fakeWhatsAppGateway
}

func TestHITLBudgetGateSuite(t *testing.T) {
	suite.Run(t, new(HITLBudgetGateSuite))
}

func (s *HITLBudgetGateSuite) SetupTest() {
	s.ctx = context.Background()
	s.wa = &fakeWhatsAppGateway{}
}

func buildBudgetConfirmDef() platform.Definition[confirmation.ConfirmState] {
	targets := map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationBudgetCommit: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			return st, nil
		},
	}
	executors := map[confirmation.OperationKind]steps.DestructiveExecutor{
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

func (s *HITLBudgetGateSuite) TestBudgetCommit_WhenComplete_SuspendsForHITL() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildBudgetConfirmDef()

	convo := &fakeBudgetConvo{
		result: tools.BudgetConversationResult{
			Complete: true,
			Draft:    budgetdraft.New("2026-06"),
			Reply:    "Orçamento completo!",
		},
	}
	committer := &fakeBudgetCommitter{reply: "ativado"}
	session := &fakeBudgetSession{}

	userID := uuid.New()
	peer := "+5511999"

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: mustBuildConfigureBudgetIntent()},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		BudgetConvo:     convo,
		BudgetCommitter: committer,
		BudgetSession:   session,
		Kernel: &services.KernelDeps{
			ConfirmEngine: confirmEngine,
			ConfirmDef:    confirmDef,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	result := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "configure o orçamento", WhatsAppTo: peer},
	)

	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.NotEmpty(result.Reply)
	s.Equal(0, committer.calls, "committer NAO deve ser chamado antes da confirmacao HITL")
}

func (s *HITLBudgetGateSuite) TestBudgetCommit_WhenConfirmed_Activates() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildBudgetConfirmDef()

	convo := &fakeBudgetConvo{
		result: tools.BudgetConversationResult{
			Complete: true,
			Draft:    budgetdraft.New("2026-06"),
			Reply:    "Orçamento completo!",
		},
	}
	committer := &fakeBudgetCommitter{reply: "ativado"}
	session := &fakeBudgetSession{}

	userID := uuid.New()
	peer := "+5511999"

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: mustBuildConfigureBudgetIntent()},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		BudgetConvo:     convo,
		BudgetCommitter: committer,
		BudgetSession:   session,
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
		services.InboundMessage{Text: "configure o orçamento", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeClarify, first.Outcome)
	s.Equal(0, committer.calls, "committer NAO deve ser chamado antes da confirmacao")

	second := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "sim", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeRouted, second.Outcome)
	s.Contains(second.Reply, "ativado")
}

func (s *HITLBudgetGateSuite) TestBudgetCommit_WhenCancelled_PreservesBudget() {
	obs := fake.NewProvider()
	store := newE2EStore()
	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, obs)
	confirmDef := buildBudgetConfirmDef()

	convo := &fakeBudgetConvo{
		result: tools.BudgetConversationResult{
			Complete: true,
			Draft:    budgetdraft.New("2026-06"),
			Reply:    "Orçamento completo!",
		},
	}
	committer := &fakeBudgetCommitter{reply: "ativado"}
	session := &fakeBudgetSession{}

	userID := uuid.New()
	peer := "+5511999"

	deps := services.IntentRouterDeps{
		Parser:          &fakeParser{intent: mustBuildConfigureBudgetIntent()},
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		BudgetConvo:     convo,
		BudgetCommitter: committer,
		BudgetSession:   session,
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
		services.InboundMessage{Text: "configure o orçamento", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeClarify, first.Outcome)

	second := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: userID},
		services.InboundMessage{Text: "não", WhatsAppTo: peer},
	)
	s.Equal(tools.OutcomeRouted, second.Outcome)
	s.Equal(0, committer.calls, "committer NAO deve ser chamado quando cancelado")
	s.Contains(second.Reply, "cancelad")
}

func mustBuildConfigureBudgetIntent() intent.Intent {
	return intent.NewConfigureBudget()
}
