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

func (s *BudgetManageWorkflowSuite) TestBuildBudgetManageReaper() {
	reaper := BuildBudgetManageReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}
