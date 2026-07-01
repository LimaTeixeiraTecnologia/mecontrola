package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type OnboardingWorkflowSuite struct {
	suite.Suite
	ctx         context.Context
	agentMock   *agentmocks.Agent
	cardsMock   *interfacemocks.CardManager
	budgetsMock *interfacemocks.BudgetPlanner
	wmMock      *memorymocks.WorkingMemory
}

func TestOnboardingWorkflowSuite(t *testing.T) {
	suite.Run(t, new(OnboardingWorkflowSuite))
}

func (s *OnboardingWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.agentMock = agentmocks.NewAgent(s.T())
	s.cardsMock = interfacemocks.NewCardManager(s.T())
	s.budgetsMock = interfacemocks.NewBudgetPlanner(s.T())
	s.wmMock = memorymocks.NewWorkingMemory(s.T())
}

func (s *OnboardingWorkflowSuite) TestDecideGoal() {
	type args struct {
		text string
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(goal string, err error)
	}{
		{
			name: "deve retornar erro para texto vazio",
			args: args{text: ""},
			expect: func(goal string, err error) {
				s.Error(err)
				s.Empty(goal)
			},
		},
		{
			name: "deve retornar erro para texto apenas espacos",
			args: args{text: "   "},
			expect: func(goal string, err error) {
				s.Error(err)
				s.Empty(goal)
			},
		},
		{
			name: "deve retornar goal trimado com sucesso",
			args: args{text: "  economizar 20% do salario  "},
			expect: func(goal string, err error) {
				s.NoError(err)
				s.Equal("economizar 20% do salario", goal)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			goal, err := DecideGoal(scenario.args.text)
			scenario.expect(goal, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestDecideIncomeCents() {
	type args struct {
		amountBRL float64
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(cents int64, err error)
	}{
		{
			name:   "deve retornar erro para valor zero",
			args:   args{amountBRL: 0},
			expect: func(cents int64, err error) { s.Error(err); s.Zero(cents) },
		},
		{
			name:   "deve retornar erro para valor negativo",
			args:   args{amountBRL: -100},
			expect: func(cents int64, err error) { s.Error(err); s.Zero(cents) },
		},
		{
			name:   "deve converter 3000 BRL para 300000 centavos",
			args:   args{amountBRL: 3000.00},
			expect: func(cents int64, err error) { s.NoError(err); s.Equal(int64(300000), cents) },
		},
		{
			name:   "deve arredondar fracao corretamente",
			args:   args{amountBRL: 1.5},
			expect: func(cents int64, err error) { s.NoError(err); s.Equal(int64(150), cents) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			cents, err := DecideIncomeCents(scenario.args.amountBRL)
			scenario.expect(cents, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestDecideDistribution() {
	type args struct {
		allocs map[string]int
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name:   "deve retornar erro quando categoria ausente",
			args:   args{allocs: map[string]int{"expense.custo_fixo": 100}},
			expect: func(err error) { s.Error(err) },
		},
		{
			name: "deve retornar erro quando soma diferente de 100",
			args: args{allocs: map[string]int{
				"expense.custo_fixo":           50,
				"expense.conhecimento":         10,
				"expense.prazeres":             10,
				"expense.metas":                10,
				"expense.liberdade_financeira": 10,
			}},
			expect: func(err error) { s.Error(err) },
		},
		{
			name: "deve aceitar distribuicao valida com soma 100",
			args: args{allocs: map[string]int{
				"expense.custo_fixo":           50,
				"expense.conhecimento":         10,
				"expense.prazeres":             10,
				"expense.metas":                10,
				"expense.liberdade_financeira": 20,
			}},
			expect: func(err error) { s.NoError(err) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := DecideDistribution(scenario.args.allocs)
			scenario.expect(err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestDecideCardEntry() {
	type args struct {
		nickname string
		dueDay   int
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name:   "deve retornar erro para nickname vazio",
			args:   args{nickname: "", dueDay: 10},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve retornar erro para dueDay zero",
			args:   args{nickname: "Nubank", dueDay: 0},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve retornar erro para dueDay 32",
			args:   args{nickname: "Nubank", dueDay: 32},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve aceitar entry valida",
			args:   args{nickname: "Nubank", dueDay: 10},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name:   "deve aceitar dueDay limite 31",
			args:   args{nickname: "Bradesco", dueDay: 31},
			expect: func(err error) { s.NoError(err) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := DecideCardEntry(scenario.args.nickname, scenario.args.dueDay)
			scenario.expect(err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestParseOnboardingPhase() {
	type args struct {
		s string
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(phase OnboardingPhase, err error)
	}{
		{
			name:   "deve parsear welcome",
			args:   args{s: "welcome"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseWelcome, phase) },
		},
		{
			name:   "deve parsear conclusion",
			args:   args{s: "conclusion"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseConclusion, phase) },
		},
		{
			name:   "deve retornar erro para fase invalida",
			args:   args{s: "invalid"},
			expect: func(phase OnboardingPhase, err error) { s.Error(err); s.Equal(OnboardingPhase(0), phase) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			phase, err := ParseOnboardingPhase(scenario.args.s)
			scenario.expect(phase, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildWelcomeStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira chamada deve suspender com mensagem de boas-vindas",
			args: args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					stream := &fakeResultStream{deltas: []string{"Bem-vindo!"}}
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal("Bem-vindo!", out.Suspend.Prompt)
				s.Equal(PhaseWelcome, out.State.Phase)
			},
		},
		{
			name: "resume deve completar limpando ResumeText",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "oi"}},
			dependencies: dependencies{
				agentMock: s.agentMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "deve falhar quando agent stream retorna erro",
			args: args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(nil, errors.New("stream error")).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepWelcomeID, BuildWelcomeStep(scenario.dependencies.agentMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildGoalStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira chamada deve suspender com pergunta sobre objetivo",
			args: args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					stream := &fakeResultStream{deltas: []string{"Qual seu objetivo?"}}
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal("Qual seu objetivo?", out.Suspend.Prompt)
			},
		},
		{
			name: "resume com objetivo valido deve completar e definir Goal",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "quero economizar 20%"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalExtract{Goal: "economizar 20% do salario"})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal("economizar 20% do salario", out.State.Goal)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume com objetivo vazio deve falhar",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "..."}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalExtract{Goal: ""})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepGoalID, BuildGoalStep(scenario.dependencies.agentMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildMethodologyStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}

	validDistrib := distributionExtract{
		CustoFixo: 50, Conhecimento: 10, Prazeres: 10, Metas: 10, LiberdadeFinanceira: 20,
	}
	invalidDistrib := distributionExtract{
		CustoFixo: 10, Conhecimento: 10, Prazeres: 10, Metas: 10, LiberdadeFinanceira: 10,
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira chamada deve suspender com mensagem sobre metodologia",
			args: args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					stream := &fakeResultStream{deltas: []string{"Metodologia explicada."}}
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
			},
		},
		{
			name: "resume com distribuicao valida deve completar e definir Allocations",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "50 10 10 10 20"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(validDistrib)
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.NotEmpty(out.State.Allocations)
				s.Equal(50, out.State.Allocations["expense.custo_fixo"])
			},
		},
		{
			name: "resume com distribuicao invalida deve re-suspender com reask",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "10 10 10 10 10"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(invalidDistrib)
					stream := &fakeResultStream{deltas: []string{"Tente novamente!"}}
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal("Tente novamente!", out.Suspend.Prompt)
				s.Empty(out.State.ResumeText)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepMethodologyID, BuildMethodologyStep(scenario.dependencies.agentMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildSummaryStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira chamada deve suspender com resumo e pedido de confirmacao",
			args: args{state: OnboardingState{
				UserID: "u1", Goal: "economizar", IncomeCents: 300000,
				Allocations: map[string]int{"expense.custo_fixo": 100},
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					stream := &fakeResultStream{deltas: []string{"Confirma o resumo?"}}
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal("Confirma o resumo?", out.Suspend.Prompt)
				s.Equal(PhaseSummary, out.State.Phase)
			},
		},
		{
			name: "resume com confirmacao deve completar",
			args: args{state: OnboardingState{UserID: "u1", Goal: "economizar", ResumeText: "sim, confirmo"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(yesNoExtract{Confirmed: true})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume sem confirmacao deve re-suspender com reask",
			args: args{state: OnboardingState{UserID: "u1", Goal: "economizar", ResumeText: "nao, quero revisar"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(yesNoExtract{Confirmed: false})
					stream := &fakeResultStream{deltas: []string{"Deseja revisar algo?"}}
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal("Deseja revisar algo?", out.Suspend.Prompt)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "deve falhar quando execute retorna erro",
			args: args{state: OnboardingState{UserID: "u1", Goal: "economizar", ResumeText: "sim"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{}, errors.New("llm error")).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepSummaryID, BuildSummaryStep(scenario.dependencies.agentMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildDistributionStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		budgetsMock *interfacemocks.BudgetPlanner
	}
	baseState := OnboardingState{
		UserID:      "11111111-1111-1111-1111-111111111111",
		IncomeCents: 300000,
		Allocations: map[string]int{"expense.custo_fixo": 100},
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "sem orcamento pre-existente deve criar e completar",
			args: args{state: baseState},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(interfaces.BudgetSummary{}, interfaces.ErrBudgetNotFound).Once()
					s.budgetsMock.EXPECT().
						CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
						Return(interfaces.BudgetRef{}, nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
			},
		},
		{
			name: "orcamento pre-existente em draft deve deletar e recriar",
			args: args{state: baseState},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(interfaces.BudgetSummary{State: "draft"}, nil).Once()
					s.budgetsMock.EXPECT().
						DeleteDraftBudget(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(nil).Once()
					s.budgetsMock.EXPECT().
						CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
						Return(interfaces.BudgetRef{}, nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
			},
		},
		{
			name: "orcamento pre-existente ativo deve reutilizar sem criar",
			args: args{state: baseState},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(interfaces.BudgetSummary{State: "active"}, nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepDistributionID, BuildDistributionStep(scenario.dependencies.budgetsMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_ActivateAlreadyActiveIsIdempotent() {
	state := OnboardingState{UserID: "11111111-1111-1111-1111-111111111111", Goal: "economizar"}
	s.budgetsMock.EXPECT().
		ActivateBudget(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
		Return(interfaces.ErrBudgetAlreadyActive).Once()
	stream := &fakeResultStream{deltas: []string{"Orcamento ativado! Deseja recorrencia?"}}
	s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
	s.agentMock.EXPECT().
		Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(stream, nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.agentMock, s.budgetsMock, s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.NotEqual(workflow.StepStatusFailed, out.Status)
	s.Equal(workflow.StepStatusSuspended, out.Status)
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_AffirmativeCreatesRecurrence() {
	state := OnboardingState{
		UserID:     "11111111-1111-1111-1111-111111111111",
		Goal:       "economizar",
		ResumeText: "sim",
	}
	payload, _ := json.Marshal(yesNoExtract{Confirmed: true})
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()
	s.budgetsMock.EXPECT().
		CreateRecurrence(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), 12).
		Return(nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, mock.AnythingOfType("string")).
		Return(nil).Once()
	stream := &fakeResultStream{deltas: []string{"Parabens pelo onboarding!"}}
	s.agentMock.EXPECT().ID().Return("onboarding-agent").Once()
	s.agentMock.EXPECT().
		Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(stream, nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.agentMock, s.budgetsMock, s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.True(out.State.Recurrence)
}

func (s *OnboardingWorkflowSuite) TestBuildOnboardingWorkflow_IDAndStructure() {
	s.agentMock.EXPECT().ID().Return("onboarding-agent").Maybe()
	def := BuildOnboardingWorkflow(s.agentMock, s.cardsMock, s.budgetsMock, s.wmMock)
	s.Equal(OnboardingWorkflowID, def.ID)
	s.NotNil(def.Root)
	s.True(def.Durable)
	s.Equal(3, def.MaxAttempts)
}

type fakeResultStream struct {
	deltas []string
}

func (f *fakeResultStream) Deltas() <-chan string {
	ch := make(chan string, len(f.deltas))
	for _, d := range f.deltas {
		ch <- d
	}
	close(ch)
	return ch
}

func (f *fakeResultStream) Result(_ context.Context) (agentpkg.Result, error) {
	content := ""
	for _, d := range f.deltas {
		content += d
	}
	return agentpkg.Result{Content: content}, nil
}
