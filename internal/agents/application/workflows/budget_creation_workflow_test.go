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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type BudgetCreationWorkflowSuite struct {
	suite.Suite
	ctx         context.Context
	agentMock   *agentmocks.Agent
	budgetsMock *interfacemocks.BudgetPlanner
	userID      uuid.UUID
}

func TestBudgetCreationWorkflowSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationWorkflowSuite))
}

func (s *BudgetCreationWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.agentMock = agentmocks.NewAgent(s.T())
	s.budgetsMock = interfacemocks.NewBudgetPlanner(s.T())
	s.userID = uuid.New()
}

func (s *BudgetCreationWorkflowSuite) TestBuildBudgetCreationWorkflow_Definition() {
	def := BuildBudgetCreationWorkflow(s.agentMock, s.budgetsMock)
	s.Equal(BudgetCreationWorkflowID, def.ID)
	s.True(def.Durable)
	s.Equal(1, def.MaxAttempts)
	s.NotNil(def.Root)
}

func (s *BudgetCreationWorkflowSuite) TestHandleBudgetTotalSlot() {
	type args struct {
		state BudgetCreationState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[BudgetCreationState], err error)
	}{
		{
			name: "primeira mensagem deve suspender pedindo o total",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetTotal}},
			dependencies: dependencies{
				agentMock: s.agentMock,
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Contains(out.Suspend.Prompt, "valor total")
			},
		},
		{
			name: "total valido deve avancar direto para a suspensao de distribuicao",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetTotal, ResumeText: "3500"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(incomeExtract{AmountBRL: 3500})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(int64(350000), out.State.TotalCents)
				s.Equal(AwaitingBudgetDistribution, out.State.Awaiting)
				s.Contains(out.Suspend.Prompt, "distribuir")
			},
		},
		{
			name: "total invalido deve reperguntar",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetTotal, ResumeText: "não sei"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(incomeExtract{AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(AwaitingBudgetTotal, out.State.Awaiting)
				s.Contains(out.Suspend.Prompt, "Não consegui identificar")
			},
		},
		{
			name: "falha do agent deve falhar o step",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetTotal, ResumeText: "3500"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{}, errors.New("llm indisponivel")).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			def := BuildBudgetCreationWorkflow(scenario.dependencies.agentMock, s.budgetsMock)
			out, err := def.Root.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *BudgetCreationWorkflowSuite) TestHandleBudgetDistributionSlot() {
	type args struct {
		state BudgetCreationState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[BudgetCreationState], err error)
	}{
		{
			name: "sem resume deve suspender oferecendo o default",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetDistribution, TotalCents: 350000}},
			dependencies: dependencies{
				agentMock: s.agentMock,
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Contains(out.Suspend.Prompt, "sugestão padrão")
			},
		},
		{
			name: "aceitar sugestao (confirm) deve aplicar default e transitar para confirmacao",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetDistribution, TotalCents: 350000, ResumeText: "sim, aceito"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{Action: "confirm"})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(AwaitingBudgetConfirm, out.State.Awaiting)
				s.Equal(10000, sumAllocations(out.State.Allocations))
				s.Contains(out.Suspend.Prompt, "Posso ativar")
			},
		},
		{
			name: "distribuicao em percentual que soma 100 deve transitar",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetDistribution, TotalCents: 350000, ResumeText: "40 10 10 10 30"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{
						Action:              "percent",
						CustoFixo:           40,
						Conhecimento:        10,
						Prazeres:            10,
						Metas:               10,
						LiberdadeFinanceira: 30,
					})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(AwaitingBudgetConfirm, out.State.Awaiting)
				s.Equal(10000, sumAllocations(out.State.Allocations))
			},
		},
		{
			name: "distribuicao que nao fecha 100 deve reperguntar sem transitar",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetDistribution, TotalCents: 350000, ResumeText: "40 10 10 10 20"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{
						Action:              "percent",
						CustoFixo:           40,
						Conhecimento:        10,
						Prazeres:            10,
						Metas:               10,
						LiberdadeFinanceira: 20,
					})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(AwaitingBudgetDistribution, out.State.Awaiting)
				s.Contains(out.Suspend.Prompt, "não consegui aplicar essa distribuição")
			},
		},
		{
			name: "acao nao reconhecida deve reperguntar sem transitar",
			args: args{state: BudgetCreationState{UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetDistribution, TotalCents: 350000, ResumeText: "???"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{Action: "unknown"})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(AwaitingBudgetDistribution, out.State.Awaiting)
				s.Contains(out.Suspend.Prompt, "não entendi sua resposta")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			def := BuildBudgetCreationWorkflow(scenario.dependencies.agentMock, s.budgetsMock)
			out, err := def.Root.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func sumAllocations(allocations map[string]int) int {
	total := 0
	for _, bp := range allocations {
		total += bp
	}
	return total
}

func (s *BudgetCreationWorkflowSuite) TestHandleBudgetConfirmSlot() {
	type args struct {
		state BudgetCreationState
	}
	type dependencies struct {
		budgetsMock *interfacemocks.BudgetPlanner
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[BudgetCreationState], err error)
	}{
		{
			name: "sem resume deve suspender pedindo confirmacao",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(),
			}},
			dependencies: dependencies{budgetsMock: s.budgetsMock},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Contains(out.Suspend.Prompt, "Posso ativar")
			},
		},
		{
			name: "confirmacao sim deve criar e ativar o orcamento",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(), ResumeText: "sim",
			}},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
						Return(interfaces.BudgetRef{ID: "budget-1", Competence: "2026-06", State: "draft"}, nil).Once()
					s.budgetsMock.EXPECT().
						ActivateBudget(mock.Anything, s.userID, "2026-06").
						Return(nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(BudgetCreationCompleted, out.State.Status)
				s.Contains(out.State.ResponseText, "criado e ativado")
				s.Contains(out.State.ResponseText, "junho de 2026")
			},
		},
		{
			name: "conflito de orcamento existente nao duplica",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(), ResumeText: "sim",
			}},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
						Return(interfaces.BudgetRef{}, interfaces.ErrBudgetConflict).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(BudgetCreationCompleted, out.State.Status)
				s.Contains(out.State.ResponseText, "Já existe um orçamento")
				s.Contains(out.State.ResponseText, "junho de 2026")
			},
		},
		{
			name: "falha real de criacao deve falhar o step com mensagem especifica",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(), ResumeText: "sim",
			}},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
						Return(interfaces.BudgetRef{}, errors.New("db indisponivel")).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
				s.Contains(out.State.ResponseText, "Não consegui criar o orçamento")
			},
		},
		{
			name: "confirmacao nao deve cancelar sem persistir",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(), ResumeText: "não",
			}},
			dependencies: dependencies{budgetsMock: s.budgetsMock},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(BudgetCreationCancelled, out.State.Status)
			},
		},
		{
			name: "primeira resposta ambigua deve reperguntar uma vez",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(), ResumeText: "talvez",
			}},
			dependencies: dependencies{budgetsMock: s.budgetsMock},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(1, out.State.RepromptCount)
			},
		},
		{
			name: "segunda resposta ambigua deve cancelar",
			args: args{state: BudgetCreationState{
				UserID: s.userID, Competence: "2026-06", Awaiting: AwaitingBudgetConfirm,
				TotalCents: 350000, Allocations: cloneDefaultBP(), ResumeText: "talvez", RepromptCount: 1,
			}},
			dependencies: dependencies{budgetsMock: s.budgetsMock},
			expect: func(out workflow.StepOutput[BudgetCreationState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(BudgetCreationCancelled, out.State.Status)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			def := BuildBudgetCreationWorkflow(s.agentMock, scenario.dependencies.budgetsMock)
			out, err := def.Root.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func cloneDefaultBP() map[string]int {
	out := make(map[string]int, len(defaultDistributionBP))
	for k, v := range defaultDistributionBP {
		out[k] = v
	}
	return out
}

func (s *BudgetCreationWorkflowSuite) TestBuildBudgetCreationReaper() {
	reaper := BuildBudgetCreationReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}
