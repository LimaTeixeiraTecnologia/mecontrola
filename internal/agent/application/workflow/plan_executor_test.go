package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PlanExecutorSuite struct {
	suite.Suite
	ctx  context.Context
	obs  observability.Observability
	user uuid.UUID
}

func TestPlanExecutorSuite(t *testing.T) {
	suite.Run(t, new(PlanExecutorSuite))
}

func (s *PlanExecutorSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.user = uuid.New()
}

func (s *PlanExecutorSuite) newEngine() platform.Engine[PlanState] {
	return platform.NewEngine[PlanState](newKernelStore(), s.obs)
}

func (s *PlanExecutorSuite) makeExpenseIntent() intent.Intent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: 3000, Merchant: "Padaria", CategoryHint: "Alimentação"})
	s.Require().NoError(err)
	return in
}

func (s *PlanExecutorSuite) makeIncomeIntent() intent.Intent {
	in, err := intent.NewRecordIncome(intent.RecordIncomeFields{AmountCents: 500000, Source: "Freelance", CategoryHint: "Receita"})
	s.Require().NoError(err)
	return in
}

func (s *PlanExecutorSuite) TestNewPlanExecutorNilEngine() {
	_, err := NewPlanExecutor(nil, func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
		return tools.ToolResult{}, nil
	}, s.obs)
	s.Error(err)
}

func (s *PlanExecutorSuite) TestNewPlanExecutorNilDispatcher() {
	_, err := NewPlanExecutor(s.newEngine(), nil, s.obs)
	s.Error(err)
}

func (s *PlanExecutorSuite) TestNewPlanExecutorNilO11y() {
	_, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
		return tools.ToolResult{}, nil
	}, nil)
	s.Error(err)
}

func (s *PlanExecutorSuite) TestEmptyPlanReturnsError() {
	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
		return tools.ToolResult{}, nil
	}, s.obs)
	s.Require().NoError(err)

	_, execErr := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-empty",
		Text:      "test",
		Plan:      PlanSteps{Steps: nil},
	})
	s.Error(execErr)
}

func (s *PlanExecutorSuite) TestSingleStepPlan() {
	dispatched := 0
	var capturedIndex int
	expense := s.makeExpenseIntent()

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		dispatched++
		capturedIndex = in.StepIndex
		return tools.ToolResult{Reply: "Despesa registrada ✅", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	result, err := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-single",
		Text:      "gastei 30 na padaria",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense, Confidence: 0.95, Index: 0},
		}},
	})

	s.NoError(err)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(result.Reply, "Despesa registrada")
	s.Equal(1, dispatched)
	s.Equal(0, capturedIndex)
}

func (s *PlanExecutorSuite) TestMultiStepPlanExecutedInOrder() {
	order := make([]int, 0, 2)
	expense := s.makeExpenseIntent()
	income := s.makeIncomeIntent()

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		order = append(order, in.StepIndex)
		reply := "ok-" + in.Intent.Kind().String()
		return tools.ToolResult{Reply: reply, Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	result, err := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-multi",
		Text:      "gastei 30 na padaria e recebi 5000 de freelance",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense, Confidence: 0.9, Index: 0},
			{Intent: income, Confidence: 0.85, Index: 1},
		}},
	})

	s.NoError(err)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal([]int{0, 1}, order)
	s.Contains(result.Reply, "ok-record_expense")
	s.Contains(result.Reply, "ok-record_income")
}

func (s *PlanExecutorSuite) TestShortCircuitOnWriteFailure() {
	callCount := 0
	expense1 := s.makeExpenseIntent()
	expense2 := s.makeExpenseIntent()

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		callCount++
		if in.StepIndex == 0 {
			return tools.ToolResult{Reply: "falha ao registrar", Outcome: tools.OutcomeUsecaseError}, nil
		}
		return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	result, err := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-fail",
		Text:      "gastei 30 duas vezes",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense1, Confidence: 0.9, Index: 0},
			{Intent: expense2, Confidence: 0.9, Index: 1},
		}},
	})

	s.NoError(err)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Equal(1, callCount)
	s.Contains(result.Reply, "falha ao registrar")
}

func (s *PlanExecutorSuite) TestDispatcherErrorShortCircuits() {
	callCount := 0
	expense1 := s.makeExpenseIntent()
	expense2 := s.makeExpenseIntent()

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		callCount++
		if in.StepIndex == 0 {
			return tools.ToolResult{}, errors.New("infra failure")
		}
		return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	_, execErr := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-dispatch-err",
		Text:      "gastei 30 duas vezes",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense1, Confidence: 0.9, Index: 0},
			{Intent: expense2, Confidence: 0.9, Index: 1},
		}},
	})

	s.Error(execErr)
	s.Equal(1, callCount)
}

func (s *PlanExecutorSuite) TestDeterministicReplyAggregation() {
	expense := s.makeExpenseIntent()
	income := s.makeIncomeIntent()
	replies := []string{"Despesa registrada ✅", "Receita registrada ✅"}
	idx := 0

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
		r := replies[idx]
		idx++
		return tools.ToolResult{Reply: r, Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	result, err := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-agg",
		Text:      "gastei e recebi",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense, Confidence: 0.9, Index: 0},
			{Intent: income, Confidence: 0.85, Index: 1},
		}},
	})

	s.NoError(err)
	s.Equal("Despesa registrada ✅\n\nReceita registrada ✅", result.Reply)
}

func (s *PlanExecutorSuite) TestReadOnlyPlanNotDurable() {
	readIntent := intent.NewHowAmIDoing()
	capturedDef := platform.Definition[PlanState]{}

	engine := &inspectEngine{inner: s.newEngine(), onStart: func(def platform.Definition[PlanState]) {
		capturedDef = def
	}}

	pe, err := NewPlanExecutor(engine, func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
		return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	_, err = pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-read",
		Text:      "como estou?",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: readIntent, Confidence: 0.9, Index: 0},
		}},
	})
	s.NoError(err)
	s.False(capturedDef.Durable)
}

func (s *PlanExecutorSuite) TestStepIndexPassedToDispatcher() {
	expense := s.makeExpenseIntent()
	income := s.makeIncomeIntent()
	capturedIndexes := make([]int, 0, 2)

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		capturedIndexes = append(capturedIndexes, in.StepIndex)
		return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	_, err = pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-idx",
		Text:      "multi",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense, Confidence: 0.9, Index: 0},
			{Intent: income, Confidence: 0.85, Index: 1},
		}},
	})

	s.NoError(err)
	s.Equal([]int{0, 1}, capturedIndexes)
}

func (s *PlanExecutorSuite) TestSerializeDeserializeByRefAndConfigureBudget() {
	deleteByRef, err := intent.NewDeleteTransactionByRef("uber")
	s.Require().NoError(err)
	editByRef, err := intent.NewEditTransactionByRef("mercado", 4200)
	s.Require().NoError(err)
	configureBudget, err := intent.NewConfigureBudget(intent.ConfigureBudgetFields{
		TotalCents:  400000,
		Allocations: map[string]int{"expense.metas": 4000},
	})
	s.Require().NoError(err)

	captured := make([]intent.Intent, 0, 3)
	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		captured = append(captured, in.Intent)
		return tools.ToolResult{Reply: in.Intent.Kind().String(), Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	result, err := pe.Execute(s.ctx, PlanInput{
		UserID:    s.user,
		Channel:   "whatsapp",
		MessageID: "msg-by-ref",
		Text:      "plano",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: deleteByRef, Confidence: 0.9, Index: 0},
			{Intent: editByRef, Confidence: 0.9, Index: 1},
			{Intent: configureBudget, Confidence: 0.9, Index: 2},
		}},
	})

	s.NoError(err)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(result.Reply, "delete_transaction_by_ref")
	s.Contains(result.Reply, "edit_transaction_by_ref")
	s.Contains(result.Reply, "configure_budget")
	s.Require().Len(captured, 3)
	s.Equal("uber", captured[0].SearchQuery())
	s.Equal("mercado", captured[1].SearchQuery())
	s.Equal(int64(4200), captured[1].AmountCents())
	s.Equal(int64(400000), captured[2].BudgetTotalCents())
	s.Equal(map[string]int{"expense.metas": 4000}, captured[2].BudgetAllocations())
}

func (s *PlanExecutorSuite) TestRunConflictReturnsReplay() {
	cases := []struct {
		name string
		err  error
	}{
		{"already_exists", platform.ErrRunAlreadyExists},
		{"conflict", platform.ErrRunConflict},
	}

	for _, tc := range cases {
		s.Run(tc.name+"/execute", func() {
			engine := &conflictEngine{err: tc.err}
			pe, err := NewPlanExecutor(engine, func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
				return tools.ToolResult{Outcome: tools.OutcomeRouted}, nil
			}, s.obs)
			s.Require().NoError(err)

			result, execErr := pe.Execute(s.ctx, PlanInput{
				UserID:    s.user,
				Channel:   "whatsapp",
				MessageID: "msg-conflict-" + tc.name,
				Text:      "gastei 30",
				Plan: PlanSteps{Steps: []PlanStepItem{
					{s.makeExpenseIntent(), 0.9, 0},
				}},
			})

			s.NoError(execErr)
			s.Equal(tools.OutcomeReplay, result.Outcome)
		})

		s.Run(tc.name+"/resume", func() {
			engine := &conflictEngine{err: tc.err}
			pe, err := NewPlanExecutor(engine, func(_ context.Context, _ PlanDispatchInput) (tools.ToolResult, error) {
				return tools.ToolResult{Outcome: tools.OutcomeRouted}, nil
			}, s.obs)
			s.Require().NoError(err)

			result, handled, resumeErr := pe.Resume(s.ctx, s.user, "whatsapp", "sim")

			s.NoError(resumeErr)
			s.True(handled)
			s.Equal(tools.OutcomeReplay, result.Outcome)
		})
	}
}

func (s *PlanExecutorSuite) TestPlanMetadataPassedToDispatcher() {
	expense := s.makeExpenseIntent()
	var captured PlanDispatchInput

	pe, err := NewPlanExecutor(s.newEngine(), func(_ context.Context, in PlanDispatchInput) (tools.ToolResult, error) {
		captured = in
		return tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted}, nil
	}, s.obs)
	s.Require().NoError(err)

	_, err = pe.Execute(s.ctx, PlanInput{
		UserID:       s.user,
		Channel:      "whatsapp",
		MessageID:    "msg-meta",
		Text:         "multi",
		LLMModel:     "openai/gpt-4.1-mini",
		PromptSHA256: "abc123",
		DirectReply:  "oi",
		RawResponse:  "{\"kind\":\"record_expense\"}",
		Plan: PlanSteps{Steps: []PlanStepItem{
			{Intent: expense, Confidence: 0.9, Index: 3},
		}},
	})

	s.NoError(err)
	s.Equal(3, captured.StepIndex)
	s.Equal("openai/gpt-4.1-mini", captured.LLMModel)
	s.Equal("abc123", captured.PromptSHA256)
	s.Equal("oi", captured.DirectReply)
	s.Equal("{\"kind\":\"record_expense\"}", captured.RawResponse)
}

type inspectEngine struct {
	inner   platform.Engine[PlanState]
	onStart func(def platform.Definition[PlanState])
}

func (e *inspectEngine) Start(ctx context.Context, def platform.Definition[PlanState], key string, initial PlanState) (platform.RunResult[PlanState], error) {
	if e.onStart != nil {
		e.onStart(def)
	}
	return e.inner.Start(ctx, def, key, initial)
}

func (e *inspectEngine) Resume(ctx context.Context, def platform.Definition[PlanState], key string, resume []byte) (platform.RunResult[PlanState], error) {
	return e.inner.Resume(ctx, def, key, resume)
}

type conflictEngine struct {
	err error
}

func (e *conflictEngine) Start(ctx context.Context, def platform.Definition[PlanState], key string, initial PlanState) (platform.RunResult[PlanState], error) {
	return platform.RunResult[PlanState]{}, e.err
}

func (e *conflictEngine) Resume(ctx context.Context, def platform.Definition[PlanState], key string, resume []byte) (platform.RunResult[PlanState], error) {
	return platform.RunResult[PlanState]{}, e.err
}
