package workflow

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PlanSuspendResumeSuite struct {
	suite.Suite
	ctx     context.Context
	obs     observability.Observability
	store   *kernelStore
	user    uuid.UUID
	channel string
}

func TestPlanSuspendResumeSuite(t *testing.T) {
	suite.Run(t, new(PlanSuspendResumeSuite))
}

func (s *PlanSuspendResumeSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.store = newKernelStore()
	s.user = uuid.New()
	s.channel = "whatsapp"
}

func (s *PlanSuspendResumeSuite) engine() platform.Engine[PlanState] {
	return platform.NewEngine[PlanState](s.store, s.obs)
}

func (s *PlanSuspendResumeSuite) destructiveIntent() intent.Intent {
	return intent.NewDeleteLastTransaction()
}

func (s *PlanSuspendResumeSuite) expenseIntent() intent.Intent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: 4200, Merchant: "Uber", CategoryHint: "Transporte"})
	s.Require().NoError(err)
	return in
}

func (s *PlanSuspendResumeSuite) readIntent() intent.Intent {
	return intent.NewHowAmIDoing()
}

func (s *PlanSuspendResumeSuite) TestDestructiveStepSuspendsWholePlanAndResumesFromCursor() {
	var order []string
	dispatch := func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		switch {
		case in.Intent.Kind() == intent.KindDeleteLastTransaction && !in.Resuming:
			order = append(order, "delete_suspend")
			return tools.ToolResult{Reply: "Confirma apagar? sim/não", Outcome: tools.OutcomeClarify}, nil
		case in.Intent.Kind() == intent.KindDeleteLastTransaction && in.Resuming:
			order = append(order, "delete_resume:"+in.ResumeText)
			return tools.ToolResult{Reply: "Apaguei ✅", Outcome: tools.OutcomeRouted}, nil
		default:
			order = append(order, "read")
			return tools.ToolResult{Reply: "Você está indo bem 📊", Outcome: tools.OutcomeRouted}, nil
		}
	}

	pe, err := NewPlanExecutor(s.engine(), dispatch, s.obs)
	s.Require().NoError(err)

	first, err := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   s.channel,
		MessageID: "wamid-1",
		Text:      "apaga o uber e quanto gastei?",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: s.destructiveIntent(), Confidence: 0.95, Index: 0},
			{Intent: s.readIntent(), Confidence: 0.9, Index: 1},
		}},
	})
	s.Require().NoError(err)
	s.Equal(tools.OutcomeClarify, first.Outcome)
	s.Contains(first.Reply, "Confirma apagar")
	s.Equal([]string{"delete_suspend"}, order)

	snap, found, loadErr := s.store.Load(s.ctx, "plan_executor", planCorrelationKey(s.user.String(), s.channel))
	s.Require().NoError(loadErr)
	s.Require().True(found)
	s.Equal(platform.RunStatusSuspended, snap.Status)
	s.Equal(0, snap.Cursor)

	resumed, handled, resumeErr := pe.Resume(s.ctx, s.user, s.channel, "sim")
	s.Require().NoError(resumeErr)
	s.True(handled)
	s.Equal(tools.OutcomeRouted, resumed.Outcome)
	s.Contains(resumed.Reply, "Apaguei")
	s.Contains(resumed.Reply, "Você está indo bem")
	s.Equal([]string{"delete_suspend", "delete_resume:sim", "read"}, order)

	final, found2, _ := s.store.Load(s.ctx, "plan_executor", planCorrelationKey(s.user.String(), s.channel))
	s.Require().True(found2)
	s.Equal(platform.RunStatusSucceeded, final.Status)
}

func (s *PlanSuspendResumeSuite) TestPolicyBlockedMidPlanShortCircuits() {
	calls := 0
	dispatch := func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		calls++
		if in.StepIndex == 0 {
			return tools.ToolResult{Reply: "registrado", Outcome: tools.OutcomeRouted}, nil
		}
		if in.StepIndex == 1 {
			return tools.ToolResult{Reply: "bloqueado por política", Outcome: tools.OutcomePolicyBlocked}, nil
		}
		return tools.ToolResult{Reply: "não deveria executar", Outcome: tools.OutcomeRouted}, nil
	}

	pe, err := NewPlanExecutor(s.engine(), dispatch, s.obs)
	s.Require().NoError(err)

	result, execErr := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   s.channel,
		MessageID: "wamid-policy",
		Text:      "tres acoes",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 0},
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 1},
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 2},
		}},
	})
	s.Require().NoError(execErr)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Equal(2, calls)
	s.Contains(result.Reply, "registrado")
	s.Contains(result.Reply, "bloqueado")
}

func (s *PlanSuspendResumeSuite) TestAuthzDeniedMidPlanShortCircuits() {
	calls := 0
	dispatch := func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		calls++
		if in.StepIndex == 0 {
			return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
		}
		return tools.ToolResult{Reply: "negado", Outcome: tools.OutcomeAuthzDenied}, nil
	}

	pe, err := NewPlanExecutor(s.engine(), dispatch, s.obs)
	s.Require().NoError(err)

	result, execErr := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   s.channel,
		MessageID: "wamid-authz",
		Text:      "duas acoes",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 0},
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 1},
		}},
	})
	s.Require().NoError(execErr)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Equal(2, calls)
}

func (s *PlanSuspendResumeSuite) TestReplayStepDoesNotDuplicateAndAdvances() {
	calls := 0
	dispatch := func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		calls++
		if in.StepIndex == 0 {
			return tools.ToolResult{Reply: "já processado", Outcome: tools.OutcomeReplay}, nil
		}
		return tools.ToolResult{Reply: "registrado agora", Outcome: tools.OutcomeRouted}, nil
	}

	pe, err := NewPlanExecutor(s.engine(), dispatch, s.obs)
	s.Require().NoError(err)

	result, execErr := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   s.channel,
		MessageID: "wamid-replay",
		Text:      "duas acoes",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 0},
			{Intent: s.expenseIntent(), Confidence: 0.9, Index: 1},
		}},
	})
	s.Require().NoError(execErr)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(2, calls)
	s.Contains(result.Reply, "já processado")
	s.Contains(result.Reply, "registrado agora")
}

func (s *PlanSuspendResumeSuite) TestResumeWithNoSuspendedPlanReturnsNotHandled() {
	pe, err := NewPlanExecutor(s.engine(), func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
		return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	_, handled, resumeErr := pe.Resume(s.ctx, s.user, s.channel, "sim")
	s.Require().NoError(resumeErr)
	s.False(handled)
}

func (s *PlanSuspendResumeSuite) TestStopConditionIsPureOverClosedOutcomeSet() {
	s.Equal(planStepAdvance, planStepDispositionFor(tools.OutcomeRouted))
	s.Equal(planStepAdvance, planStepDispositionFor(tools.OutcomeReplay))
	s.Equal(planStepAdvance, planStepDispositionFor(tools.OutcomeFallback))
	s.Equal(planStepSuspend, planStepDispositionFor(tools.OutcomeClarify))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomeUsecaseError))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomePolicyBlocked))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomeAuthzDenied))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomeMissingResolver))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomeParseError))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomeReplyFailed))
	s.Equal(planStepShortCircuit, planStepDispositionFor(tools.OutcomeEmptyText))
}
