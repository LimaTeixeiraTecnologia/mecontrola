package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	budgetsentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

type BudgetManageWorkflowSuite struct {
	suite.Suite
	ctx         context.Context
	agentMock   *agentmocks.Agent
	budgetsMock *interfacemocks.BudgetPlanner
	userID      uuid.UUID
}

func TestBudgetManageWorkflowSuite(t *testing.T) {
	suite.Run(t, new(BudgetManageWorkflowSuite))
}

func (s *BudgetManageWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.agentMock = agentmocks.NewAgent(s.T())
	s.budgetsMock = interfacemocks.NewBudgetPlanner(s.T())
	s.userID = uuid.New()
}

func (s *BudgetManageWorkflowSuite) TestBuildBudgetManageWorkflow_Definition() {
	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	s.Equal(BudgetManageWorkflowID, def.ID)
	s.True(def.Durable)
	s.Equal(1, def.MaxAttempts)
	s.NotNil(def.Root)
}

func (s *BudgetManageWorkflowSuite) TestEditTotalEntryFetchesSummaryAndSuspends() {
	totalCents := int64(300000)
	s.budgetsMock.EXPECT().
		GetMonthlySummary(mock.Anything, s.userID, "2026-07").
		Return(interfaces.BudgetSummary{TotalCents: &totalCents}, nil).Once()
	s.budgetsMock.EXPECT().
		ListFutureBudgets(mock.Anything, s.userID, "2026-07").
		Return(nil, nil).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	state := BudgetManageState{UserID: s.userID, Competence: "2026-07", Operation: BudgetManageOpEditTotal}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(BudgetManageAwaitingTotal, out.State.Awaiting)
	s.Equal(totalCents, out.State.PreviousTotalCents)
	s.Contains(out.Suspend.Prompt, "novo valor total")
}

func (s *BudgetManageWorkflowSuite) TestEditTotalConfirmExecutesEditBudgetTotal() {
	state := BudgetManageState{
		UserID:     s.userID,
		Competence: "2026-07",
		Operation:  BudgetManageOpEditTotal,
		Awaiting:   BudgetManageAwaitingConfirm,
		TotalCents: 400000,
		ResumeText: "sim",
	}

	state.ApplyScope = BudgetManageApplyScopeCurrentOnly
	s.budgetsMock.EXPECT().
		EditBudgetTotal(mock.Anything, s.userID, "2026-07", int64(400000)).
		Return(nil).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(BudgetManageCompleted, out.State.Status)
	s.Contains(out.State.ResponseText, "atualizado")
}

func (s *BudgetManageWorkflowSuite) TestEditTotalConfirmMapsDomainError() {
	state := BudgetManageState{
		UserID:     s.userID,
		Competence: "2026-07",
		Operation:  BudgetManageOpEditTotal,
		Awaiting:   BudgetManageAwaitingConfirm,
		TotalCents: 400000,
		ResumeText: "sim",
	}

	state.ApplyScope = BudgetManageApplyScopeCurrentOnly
	s.budgetsMock.EXPECT().
		EditBudgetTotal(mock.Anything, s.userID, "2026-07", int64(400000)).
		Return(budgetsentities.ErrBudgetNotActive).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "orçamento ativo")
}

func (s *BudgetManageWorkflowSuite) TestCreateRetroactiveConfirmFalseSuccessOnEmptyRef() {
	state := BudgetManageState{
		UserID:     s.userID,
		Competence: "2026-07",
		Operation:  BudgetManageOpCreateRetroactive,
		Awaiting:   BudgetManageAwaitingConfirm,
		TotalCents: 400000,
		ResumeText: "sim",
	}

	s.budgetsMock.EXPECT().
		CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
		Return(interfaces.BudgetRef{ID: ""}, nil).Once()
	s.budgetsMock.EXPECT().
		ActivateBudget(mock.Anything, s.userID, "2026-07").
		Return(nil).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	out, err := def.Root.Execute(s.ctx, state)

	s.Error(err)
	s.True(errors.Is(err, ErrBudgetManageAcceptedWithoutResource))
	s.Equal(workflow.StepStatusFailed, out.Status)
	s.NotEqual(BudgetManageCompleted, out.State.Status)
	s.NotContains(out.State.ResponseText, "criado e ativado com sucesso")
}

func (s *BudgetManageWorkflowSuite) TestCreateRetroactiveConfirmPersistsAndSucceeds() {
	state := BudgetManageState{
		UserID:     s.userID,
		Competence: "2026-07",
		Operation:  BudgetManageOpCreateRetroactive,
		Awaiting:   BudgetManageAwaitingConfirm,
		TotalCents: 400000,
		ResumeText: "sim",
	}

	s.budgetsMock.EXPECT().
		CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
		Return(interfaces.BudgetRef{ID: uuid.NewString()}, nil).Once()
	s.budgetsMock.EXPECT().
		ActivateBudget(mock.Anything, s.userID, "2026-07").
		Return(nil).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(BudgetManageCompleted, out.State.Status)
	s.Contains(out.State.ResponseText, "sucesso")
}

func (s *BudgetManageWorkflowSuite) TestConfirmCancel() {
	state := BudgetManageState{
		UserID:     s.userID,
		Competence: "2026-07",
		Operation:  BudgetManageOpEditTotal,
		Awaiting:   BudgetManageAwaitingConfirm,
		TotalCents: 400000,
		ResumeText: "não",
	}

	state.ApplyScope = BudgetManageApplyScopeCurrentOnly
	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(BudgetManageCancelled, out.State.Status)
	s.Contains(out.State.ResponseText, "cancelada")
}

func (s *BudgetManageWorkflowSuite) TestCreateRetroactiveTotalSlotAdvancesToDistribution() {
	payload, _ := json.Marshal(monthlyBudgetExtract{AmountBRL: 3500})
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	state := BudgetManageState{
		UserID:     s.userID,
		Competence: "2026-07",
		Operation:  BudgetManageOpCreateRetroactive,
		Awaiting:   BudgetManageAwaitingTotal,
		ResumeText: "3500",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(int64(350000), out.State.TotalCents)
	s.Equal(BudgetManageAwaitingDistribution, out.State.Awaiting)
}

func (s *BudgetManageWorkflowSuite) TestEditDistributionEntryWithFutureBudgetsAsksScopeAfterDistribution() {
	totalCents := int64(300000)
	s.budgetsMock.EXPECT().
		GetMonthlySummary(mock.Anything, s.userID, "2026-07").
		Return(interfaces.BudgetSummary{
			TotalCents: &totalCents,
			Allocations: []interfaces.AllocationSummary{
				{RootSlug: "expense.custo_fixo", PlannedCents: int64Ptr(150000)},
				{RootSlug: "expense.conhecimento", PlannedCents: int64Ptr(15000)},
				{RootSlug: "expense.prazeres", PlannedCents: int64Ptr(30000)},
				{RootSlug: "expense.metas", PlannedCents: int64Ptr(45000)},
				{RootSlug: "expense.liberdade_financeira", PlannedCents: int64Ptr(60000)},
			},
		}, nil).Once()
	s.budgetsMock.EXPECT().
		ListFutureBudgets(mock.Anything, s.userID, "2026-07").
		Return([]interfaces.FutureBudget{{Competence: "2026-08", State: "draft"}}, nil).Once()

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	state := BudgetManageState{UserID: s.userID, Competence: "2026-07", Operation: BudgetManageOpEditDistribution}

	out, err := def.Root.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(BudgetManageAwaitingDistribution, out.State.Awaiting)

	payload, _ := json.Marshal(allocationInputExtract{
		Action:              "percent",
		CustoFixo:           50,
		Conhecimento:        5,
		Prazeres:            10,
		Metas:               15,
		LiberdadeFinanceira: 20,
	})
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()

	state = out.State
	state.ResumeText = "custos fixos 50, conhecimento 5, prazeres 10, metas 15, liberdade financeira 20"

	out, err = def.Root.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(BudgetManageAwaitingApplyScope, out.State.Awaiting)
	s.Contains(out.Suspend.Prompt, "subsequentes")
}

func (s *BudgetManageWorkflowSuite) TestApplyScopeThenSyncFutureBudgets() {
	state := BudgetManageState{
		UserID:           s.userID,
		Competence:       "2026-07",
		Operation:        BudgetManageOpEditTotal,
		Awaiting:         BudgetManageAwaitingApplyScope,
		TotalCents:       400000,
		ResumeText:       "todos os subsequentes",
		HasFutureBudgets: true,
	}

	def := BuildBudgetManageWorkflowWithObservability(s.agentMock, s.budgetsMock, nil)
	out, err := def.Root.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(BudgetManageAwaitingConfirm, out.State.Awaiting)
	s.Equal(BudgetManageApplyScopeCurrentAndSubsequent, out.State.ApplyScope)

	state = out.State
	state.ResumeText = "sim"
	s.budgetsMock.EXPECT().
		EditBudgetTotal(mock.Anything, s.userID, "2026-07", int64(400000)).
		Return(nil).Once()
	s.budgetsMock.EXPECT().
		SyncFutureBudgets(mock.Anything, s.userID, "2026-07").
		Return(interfaces.FutureBudgetSyncResult{UpdatedCompetences: []string{"2026-08"}}, nil).Once()

	out, err = def.Root.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "subsequentes")
}

func int64Ptr(v int64) *int64 {
	return &v
}

func (s *BudgetManageWorkflowSuite) TestBuildBudgetManageReaper() {
	reaper := BuildBudgetManageReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}
