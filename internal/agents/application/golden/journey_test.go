package golden

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/reconciliation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type JourneyGoldenSuite struct {
	suite.Suite
	ctx context.Context
}

func TestJourneyGoldenSuite(t *testing.T) {
	suite.Run(t, new(JourneyGoldenSuite))
}

func (s *JourneyGoldenSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *JourneyGoldenSuite) TestJourneyCasesAreRegisteredAndValid() {
	cases := journeyCases()
	s.Require().NotEmpty(cases)
	s.Require().NoError(ValidateAll(cases))

	registered := AllCases()
	names := make(map[string]bool, len(registered))
	for _, c := range registered {
		names[c.Name] = true
	}
	for _, c := range cases {
		s.Truef(names[c.Name], "caso de jornada %q deve estar registrado em AllCases", c.Name)
	}
}

func (s *JourneyGoldenSuite) TestJourneyCategoriesCovered() {
	journeyCategories := []Category{
		CategoryOnboarding, CategoryBudget, CategoryExpenseIncome, CategoryPending, CategoryConfirmation,
		CategoryQuery, CategoryCard, CategoryRecurrence, CategoryBudgetTotal, CategoryGoal,
		CategoryCancelPlan, CategorySupport, CategoryCategoryDetail, CategoryGeneralSummary,
	}
	for _, category := range journeyCategories {
		s.NotEmptyf(CasesByCategory(category), "categoria de jornada %q deve ter casos golden", category)
	}
}

func (s *JourneyGoldenSuite) TestJourneyGoldenIsAnonymized() {
	forbidden := []string{"3140d64a", "cf8be1b10035", "@", "+55"}
	for _, c := range journeyCases() {
		fields := []string{c.Input, c.Origin, c.ResponseDescribe}
		for _, turn := range c.PriorTurns {
			fields = append(fields, turn.UserMessage)
		}
		for _, field := range fields {
			for _, term := range forbidden {
				s.NotContainsf(field, term, "caso de jornada %q não pode conter dado pessoal/verbatim (%s)", c.Name, term)
			}
		}
	}
}

func (s *JourneyGoldenSuite) registerExpenseExecutor(amountCents float64, sink ToolCaptureSink) AgentExecutor {
	return func(_ context.Context, _ []llm.Message) (agent.Result, error) {
		sink("register_expense", map[string]any{"amountCents": amountCents})
		return agent.Result{
			Content:     "Anotado! Confirma o registro dessa despesa?",
			ToolCalls:   []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
			ToolOutcome: agent.ToolOutcomeRouted,
		}, nil
	}
}

func (s *JourneyGoldenSuite) TestInvariantNoFalseMultiItem() {
	for _, c := range journeyCases() {
		if c.Category != CategoryExpenseIncome {
			continue
		}
		s.Run(c.Name, func() {
			var captured []CapturedToolCall
			sink := func(name string, args map[string]any) {
				captured = append(captured, CapturedToolCall{Tool: name, Args: args})
			}
			amount := c.ExpectedArgs["amountCents"].(float64)
			outcome := EvaluateCaseWithCapture(s.ctx, s.registerExpenseExecutor(amount, sink), c, func() []CapturedToolCall { return captured })
			s.Truef(outcome.Passed, "despesa simples deve rotear para register_expense sem orientação de múltiplos lançamentos: %s", outcome.Detail)
		})
	}
}

func (s *JourneyGoldenSuite) TestInvariantNoDefaultBudgetOverride() {
	for _, c := range journeyCases() {
		if c.Category != CategoryBudget {
			continue
		}
		s.Run(c.Name, func() {
			executor := func(_ context.Context, _ []llm.Message) (agent.Result, error) {
				return agent.Result{
					Content:     "Vamos ajustar sua distribuição personalizada. Qual valor você quer em cada categoria?",
					ToolCalls:   []agent.ToolCallRecord{{Tool: "create_budget", Outcome: agent.ToolCallOutcomeSuccess}},
					ToolOutcome: agent.ToolOutcomeRouted,
				}, nil
			}
			outcome := EvaluateCase(s.ctx, executor, c)
			s.Truef(outcome.Passed, "personalização de orçamento não pode propor a distribuição padrão: %s", outcome.Detail)
		})
	}
}

func (s *JourneyGoldenSuite) TestInvariantNoFalseSuccessOnEmptyResource() {
	status, stepStatus, err := workflows.DecideTransactionPostWrite(agent.ToolOutcomeRouted, uuid.Nil)
	s.Require().Error(err)
	s.ErrorIs(err, workflows.ErrWriteAcceptedWithoutResource)
	s.Equal(workflow.StepStatusFailed, stepStatus)
	s.Equal(workflows.TransactionWriteStatusActive, status)
	s.NotEqual(workflows.TransactionWriteStatusCancelled, status, "escrita aceita sem recurso nunca vira Cancelled")

	okStatus, okStep, okErr := workflows.DecideTransactionPostWrite(agent.ToolOutcomeRouted, uuid.New())
	s.Require().NoError(okErr)
	s.Equal(workflow.StepStatusCompleted, okStep)
	s.Equal(workflows.TransactionWriteStatusCompleted, okStatus)
}

func (s *JourneyGoldenSuite) TestInvariantNoDuplicateConfirmationOrTransaction() {
	rows := []reconciliation.RunConsistencyRow{
		{
			RunID:             uuid.NewString(),
			CorrelationKey:    "wamid-journey-final",
			RunStatus:         agent.RunStatusSucceeded,
			Outcome:           agent.ToolOutcomeRouted,
			LedgerWrites:      1,
			TransactionCount:  1,
			WorkflowStatus:    workflow.RunStatusSucceeded,
			WorkflowFound:     true,
			WorkflowStateSet:  true,
			WorkflowStateText: workflow.RunStatusSucceeded.String(),
		},
	}
	s.Empty(reconciliation.DecideViolations(rows), "jornada com efeito durável e estados concordantes não pode gerar violação")

	falseSuccess := []reconciliation.RunConsistencyRow{
		{
			RunID:            uuid.NewString(),
			CorrelationKey:   "wamid-journey-empty",
			RunStatus:        agent.RunStatusSucceeded,
			Outcome:          agent.ToolOutcomeRouted,
			LedgerWrites:     0,
			TransactionCount: 0,
			WorkflowFound:    false,
		},
	}
	violations := reconciliation.DecideViolations(falseSuccess)
	s.Require().Len(violations, 1)
	s.Equal(reconciliation.ViolationSucceededNoEffect, violations[0].Kind, "sucesso sem efeito financeiro é detectado como violação")
}

func (s *JourneyGoldenSuite) TestInvariantOnboardingConclusionPreservesGoalSentinelAndTreatmentNameSection() {
	wm := memorymocks.NewWorkingMemory(s.T())
	budgetsMock := interfacemocks.NewBudgetPlanner(s.T())
	cardsMock := interfacemocks.NewCardManager(s.T())

	state := workflows.OnboardingState{
		UserID:        uuid.NewString(),
		Goal:          "economizar",
		TreatmentName: "Stef",
	}

	var capturedContent string
	var capturedMetadata map[string]any
	wm.EXPECT().
		Upsert(mock.Anything, state.UserID, mock.AnythingOfType("string")).
		Run(func(_ context.Context, _ string, content string) { capturedContent = content }).
		Return(nil).Once()
	wm.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, mock.Anything).
		Run(func(_ context.Context, _ string, metadata map[string]any) { capturedMetadata = metadata }).
		Return(nil).Once()
	budgetsMock.EXPECT().
		SuggestAllocation(mock.Anything, state.MonthlyBudgetCents, mock.Anything).
		Return(nil, nil).Once()
	cardsMock.EXPECT().
		ListCards(mock.Anything, uuid.MustParse(state.UserID)).
		Return(nil, nil).Once()

	conclusion := workflows.BuildConclusionStep(wm, budgetsMock, cardsMock)
	out, err := conclusion(s.ctx, state)

	s.Require().NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(capturedContent, "## Objetivo Financeiro", "sentinel de onboarding concluído (RF-13/resolve_onboarding_or_agent) deve ser preservado")
	s.Contains(capturedContent, "## Nome de Tratamento", "nova seção de nome de tratamento deve estar presente no mesmo Upsert (writer único, RF-03)")
	s.Equal(state.TreatmentName, capturedMetadata["nome_tratamento"], "metadata mirror deve refletir o nome de tratamento capturado")
}

func (s *JourneyGoldenSuite) TestInvariantOnboardingConclusionWithoutTreatmentNameOmitsSection() {
	wm := memorymocks.NewWorkingMemory(s.T())
	budgetsMock := interfacemocks.NewBudgetPlanner(s.T())
	cardsMock := interfacemocks.NewCardManager(s.T())

	state := workflows.OnboardingState{
		UserID: uuid.NewString(),
		Goal:   "economizar",
	}

	var capturedContent string
	var capturedMetadata map[string]any
	wm.EXPECT().
		Upsert(mock.Anything, state.UserID, mock.AnythingOfType("string")).
		Run(func(_ context.Context, _ string, content string) { capturedContent = content }).
		Return(nil).Once()
	wm.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, mock.Anything).
		Run(func(_ context.Context, _ string, metadata map[string]any) { capturedMetadata = metadata }).
		Return(nil).Once()
	budgetsMock.EXPECT().
		SuggestAllocation(mock.Anything, state.MonthlyBudgetCents, mock.Anything).
		Return(nil, nil).Once()
	cardsMock.EXPECT().
		ListCards(mock.Anything, uuid.MustParse(state.UserID)).
		Return(nil, nil).Once()

	conclusion := workflows.BuildConclusionStep(wm, budgetsMock, cardsMock)
	out, err := conclusion(s.ctx, state)

	s.Require().NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(capturedContent, "## Objetivo Financeiro")
	s.NotContains(capturedContent, "## Nome de Tratamento", "sem nome capturado, a seção não deve ser gravada")
	_, hasKey := capturedMetadata["nome_tratamento"]
	s.False(hasKey, "sem nome capturado, a chave nome_tratamento não deve existir no metadata")
}
