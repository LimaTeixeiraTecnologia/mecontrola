package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type OnboardingWorkflowSuite struct {
	suite.Suite
	ctx          context.Context
	agentMock    *agentmocks.Agent
	cardsMock    *interfacemocks.CardManager
	budgetsMock  *interfacemocks.BudgetPlanner
	wmMock       *memorymocks.WorkingMemory
	threadsMock  *memorymocks.ThreadGateway
	messagesMock *memorymocks.MessageStore
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
	s.threadsMock = memorymocks.NewThreadGateway(s.T())
	s.messagesMock = memorymocks.NewMessageStore(s.T())
}

func suggestReturn(income int64, bp map[string]int) []interfaces.AllocationCents {
	out := make([]interfaces.AllocationCents, 0, len(canonicalSlugs))
	for _, slug := range canonicalSlugs {
		out = append(out, interfaces.AllocationCents{
			RootSlug:     slug,
			BasisPoints:  bp[slug],
			PlannedCents: income * int64(bp[slug]) / 10000,
		})
	}
	return out
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
			name:   "deve retornar erro para texto vazio",
			args:   args{text: ""},
			expect: func(goal string, err error) { s.Error(err); s.Empty(goal) },
		},
		{
			name:   "deve retornar erro para texto apenas espacos",
			args:   args{text: "   "},
			expect: func(goal string, err error) { s.Error(err); s.Empty(goal) },
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

func (s *OnboardingWorkflowSuite) TestDecideGoalValueCents() {
	type args struct {
		hasAmount bool
		amountBRL float64
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(cents int64, ok bool)
	}{
		{
			name: "deve converter valor positivo com hasAmount true",
			args: args{hasAmount: true, amountBRL: 400000},
			expect: func(cents int64, ok bool) {
				s.True(ok)
				s.Equal(int64(40000000), cents)
			},
		},
		{
			name: "deve converter valor fracionario minimo",
			args: args{hasAmount: true, amountBRL: 0.01},
			expect: func(cents int64, ok bool) {
				s.True(ok)
				s.Equal(int64(1), cents)
			},
		},
		{
			name: "deve retornar nao informado para valor zero",
			args: args{hasAmount: true, amountBRL: 0},
			expect: func(cents int64, ok bool) {
				s.False(ok)
				s.Equal(int64(0), cents)
			},
		},
		{
			name: "deve retornar nao informado para valor negativo",
			args: args{hasAmount: true, amountBRL: -50},
			expect: func(cents int64, ok bool) {
				s.False(ok)
				s.Equal(int64(0), cents)
			},
		},
		{
			name: "deve retornar nao informado quando hasAmount false mesmo com valor positivo",
			args: args{hasAmount: false, amountBRL: 400000},
			expect: func(cents int64, ok bool) {
				s.False(ok)
				s.Equal(int64(0), cents)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			cents, ok := DecideGoalValueCents(scenario.args.hasAmount, scenario.args.amountBRL)
			scenario.expect(cents, ok)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestGoalWithValueSchema_UnmarshalsIntoExtractStruct() {
	raw := []byte(`{"goal":"comprar uma casa","hasAmount":true,"amountBRL":400000}`)
	var extract goalWithValueExtract
	err := json.Unmarshal(raw, &extract)
	s.NoError(err)
	s.Equal("comprar uma casa", extract.Goal)
	s.True(extract.HasAmount)
	s.Equal(float64(400000), extract.AmountBRL)

	s.Equal("object", goalWithValueSchema["type"])
	s.Equal(false, goalWithValueSchema["additionalProperties"])
	required, ok := goalWithValueSchema["required"].([]any)
	s.True(ok)
	s.ElementsMatch([]any{"goal", "hasAmount", "amountBRL"}, required)
	properties, ok := goalWithValueSchema["properties"].(map[string]any)
	s.True(ok)
	s.Contains(properties, "goal")
	s.Contains(properties, "hasAmount")
	s.Contains(properties, "amountBRL")
}

func (s *OnboardingWorkflowSuite) TestGoalValueSchema_UnmarshalsIntoExtractStruct() {
	raw := []byte(`{"hasAmount":false,"amountBRL":0}`)
	var extract goalValueExtract
	err := json.Unmarshal(raw, &extract)
	s.NoError(err)
	s.False(extract.HasAmount)
	s.Equal(float64(0), extract.AmountBRL)

	s.Equal("object", goalValueSchema["type"])
	s.Equal(false, goalValueSchema["additionalProperties"])
	required, ok := goalValueSchema["required"].([]any)
	s.True(ok)
	s.ElementsMatch([]any{"hasAmount", "amountBRL"}, required)
	properties, ok := goalValueSchema["properties"].(map[string]any)
	s.True(ok)
	s.Contains(properties, "hasAmount")
	s.Contains(properties, "amountBRL")
}

func (s *OnboardingWorkflowSuite) TestGoalValuePrompts_ContainMonetaryFormatExamples() {
	formatExamples := []string{
		"R$ 400.000,00",
		"400000",
		"10 mil reais",
		"400 mil",
		"1,5 milhão",
	}
	for _, example := range formatExamples {
		s.Contains(_goalWithValueSystemPrompt, example)
		s.Contains(_goalValueSystemPrompt, example)
	}
}

func (s *OnboardingWorkflowSuite) TestGoalValueReprompt_InvitesOptionalValueWithoutBlocking() {
	s.NotEmpty(_goalValueReprompt)
	s.Contains(strings.ToLower(_goalValueReprompt), "não")
}

func (s *OnboardingWorkflowSuite) TestOnboardingState_MergePatch_PreservesGoalValueFields() {
	codec := workflow.NewCodec[OnboardingState]()
	base := OnboardingState{
		Phase:          PhaseGoal,
		UserID:         "user-1",
		Goal:           "comprar uma casa",
		GoalValueCents: 40000000,
		GoalValueAsked: true,
	}

	baseBytes, err := codec.Encode(base)
	s.Require().NoError(err)

	patch := []byte(`{"resumeText":"sim, exatamente"}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	s.Require().NoError(err)

	result, err := codec.Decode(merged)
	s.Require().NoError(err)

	s.Equal("comprar uma casa", result.Goal)
	s.Equal(int64(40000000), result.GoalValueCents)
	s.True(result.GoalValueAsked)
	s.Equal("sim, exatamente", result.ResumeText)
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
			args:   args{allocs: map[string]int{"expense.custo_fixo": 10000}},
			expect: func(err error) { s.Error(err) },
		},
		{
			name: "deve retornar erro quando soma diferente de 10000",
			args: args{allocs: map[string]int{
				"expense.custo_fixo":           5000,
				"expense.conhecimento":         1000,
				"expense.prazeres":             1000,
				"expense.metas":                1000,
				"expense.liberdade_financeira": 1000,
			}},
			expect: func(err error) { s.Error(err) },
		},
		{
			name: "deve aceitar distribuicao valida com soma 10000",
			args: args{allocs: map[string]int{
				"expense.custo_fixo":           4000,
				"expense.conhecimento":         1000,
				"expense.prazeres":             1000,
				"expense.metas":                1000,
				"expense.liberdade_financeira": 3000,
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

func (s *OnboardingWorkflowSuite) TestDecideAllocationsBP() {
	type args struct {
		kind        allocationInputKind
		values      map[string]float64
		incomeCents int64
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(bp map[string]int, err error)
	}{
		{
			name: "confirm deve retornar a distribuicao oficial",
			args: args{kind: allocationInputConfirm, values: nil, incomeCents: 1350000},
			expect: func(bp map[string]int, err error) {
				s.NoError(err)
				s.Equal(4000, bp["expense.custo_fixo"])
				s.Equal(1000, bp["expense.conhecimento"])
				s.Equal(1000, bp["expense.prazeres"])
				s.Equal(1000, bp["expense.metas"])
				s.Equal(3000, bp["expense.liberdade_financeira"])
				s.NoError(DecideDistribution(bp))
			},
		},
		{
			name: "percent valido deve converter para basis points",
			args: args{kind: allocationInputPercent, values: map[string]float64{
				"expense.custo_fixo": 40, "expense.conhecimento": 10, "expense.prazeres": 10,
				"expense.metas": 10, "expense.liberdade_financeira": 30,
			}, incomeCents: 1350000},
			expect: func(bp map[string]int, err error) {
				s.NoError(err)
				s.Equal(4000, bp["expense.custo_fixo"])
				s.Equal(3000, bp["expense.liberdade_financeira"])
			},
		},
		{
			name: "percent que nao soma 100 deve retornar erro amigavel",
			args: args{kind: allocationInputPercent, values: map[string]float64{
				"expense.custo_fixo": 40, "expense.conhecimento": 10, "expense.prazeres": 10,
				"expense.metas": 10, "expense.liberdade_financeira": 20,
			}, incomeCents: 1350000},
			expect: func(bp map[string]int, err error) {
				s.Error(err)
				s.Nil(bp)
				s.Contains(err.Error(), "90%")
			},
		},
		{
			name: "reais que somam a renda deve converter para basis points",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 5400, "expense.conhecimento": 1350, "expense.prazeres": 1350,
				"expense.metas": 1350, "expense.liberdade_financeira": 4050,
			}, incomeCents: 1350000},
			expect: func(bp map[string]int, err error) {
				s.NoError(err)
				s.Equal(4000, bp["expense.custo_fixo"])
				s.Equal(1000, bp["expense.conhecimento"])
				s.Equal(3000, bp["expense.liberdade_financeira"])
				s.NoError(DecideDistribution(bp))
			},
		},
		{
			name: "reais que nao somam a renda deve retornar erro amigavel",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 5400, "expense.conhecimento": 1350, "expense.prazeres": 1350,
				"expense.metas": 1350, "expense.liberdade_financeira": 2700,
			}, incomeCents: 1350000},
			expect: func(bp map[string]int, err error) {
				s.Error(err)
				s.Nil(bp)
				s.Contains(err.Error(), "renda")
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			bp, err := DecideAllocationsBP(scenario.args.kind, scenario.args.values, scenario.args.incomeCents)
			scenario.expect(bp, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestDecideCardEntry() {
	type args struct {
		nickname string
		bank     string
		dueDay   int
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name:   "deve retornar erro para nickname vazio",
			args:   args{nickname: "", bank: "Nubank", dueDay: 10},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve retornar erro para bank vazio",
			args:   args{nickname: "Nubank", bank: "", dueDay: 10},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve retornar erro para dueDay zero",
			args:   args{nickname: "Nubank", bank: "Nubank", dueDay: 0},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve retornar erro para dueDay 32",
			args:   args{nickname: "Nubank", bank: "Nubank", dueDay: 32},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve aceitar entry valida",
			args:   args{nickname: "Nubank", bank: "Nubank", dueDay: 10},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name:   "deve aceitar dueDay limite 31",
			args:   args{nickname: "Bradesco", bank: "Bradesco", dueDay: 31},
			expect: func(err error) { s.NoError(err) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := DecideCardEntry(scenario.args.nickname, scenario.args.bank, scenario.args.dueDay)
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
			name:         "primeira mensagem deve saudar e ja perguntar o objetivo",
			args:         args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{agentMock: s.agentMock},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(_welcomeGoalPrompt, out.Suspend.Prompt)
				s.Contains(out.Suspend.Prompt, "Vamos começar?")
				s.Contains(out.Suspend.Prompt, "objetivo")
				s.Contains(out.Suspend.Prompt, "valor da meta")
				s.Contains(out.Suspend.Prompt, "R$ 400.000,00")
				s.Equal(PhaseGoal, out.State.Phase)
			},
		},
		{
			name: "meta e valor juntos devem ser extraidos em uma unica chamada sem repergunta",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "quero comprar uma casa, meta de R$ 400.000,00"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "comprar uma casa", HasAmount: true, AmountBRL: 400000})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal("comprar uma casa", out.State.Goal)
				s.Equal(int64(40000000), out.State.GoalValueCents)
				s.False(out.State.GoalValueAsked)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "meta valida sem valor deve reperguntar especificamente pelo valor",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "quero quitar minhas dividas"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "quitar minhas dividas", HasAmount: false, AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(_goalValueReprompt, out.Suspend.Prompt)
				s.Equal("quitar minhas dividas", out.State.Goal)
				s.Equal(int64(0), out.State.GoalValueCents)
				s.True(out.State.GoalValueAsked)
			},
		},
		{
			name: "sem objetivo identificavel deve reperguntar de forma combinada",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "..."}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "", HasAmount: false, AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(_goalReprompt, out.Suspend.Prompt)
				s.Empty(out.State.Goal)
				s.True(out.State.GoalValueAsked)
			},
		},
		{
			name: "resume da repergunta combinada com objetivo desta vez valido nao deve reperguntar valor de novo",
			args: args{state: OnboardingState{UserID: "u1", GoalValueAsked: true, ResumeText: "quero viajar"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "viajar", HasAmount: false, AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal("viajar", out.State.Goal)
				s.Equal(int64(0), out.State.GoalValueCents)
				s.True(out.State.GoalValueAsked)
			},
		},
		{
			name: "resume value-only com valor valido deve salvar e completar",
			args: args{state: OnboardingState{UserID: "u1", Goal: "quitar minhas dividas", GoalValueAsked: true, ResumeText: "400 mil"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalValueExtract{HasAmount: true, AmountBRL: 400000})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal("quitar minhas dividas", out.State.Goal)
				s.Equal(int64(40000000), out.State.GoalValueCents)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume value-only com recusa deve avancar sem valor",
			args: args{state: OnboardingState{UserID: "u1", Goal: "viajar", GoalValueAsked: true, ResumeText: "nao sei quanto"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalValueExtract{HasAmount: false, AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal("viajar", out.State.Goal)
				s.Equal(int64(0), out.State.GoalValueCents)
			},
		},
		{
			name:         "objetivo previo sem repergunta de valor ainda gasta deve reperguntar valor sem chamar o parser",
			args:         args{state: OnboardingState{UserID: "u1", Goal: "viajar", GoalValueAsked: false, ResumeText: "seguindo"}},
			dependencies: dependencies{agentMock: s.agentMock},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(_goalValueReprompt, out.Suspend.Prompt)
				s.Equal("viajar", out.State.Goal)
				s.Equal(int64(0), out.State.GoalValueCents)
				s.True(out.State.GoalValueAsked)
			},
		},
		{
			name: "resume da repergunta combinada ainda sem objetivo deve manter o loop de meta obrigatoria",
			args: args{state: OnboardingState{UserID: "u1", GoalValueAsked: true, ResumeText: "ainda sem meta"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "", HasAmount: false, AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(_goalReprompt, out.Suspend.Prompt)
				s.Empty(out.State.Goal)
				s.True(out.State.GoalValueAsked)
			},
		},
		{
			name: "falha do parser na extracao combinada deve falhar o step",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "quero comprar uma casa"}},
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
		{
			name: "json invalido na extracao combinada deve falhar o step",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "quero comprar uma casa"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: []byte("nao-e-json")}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
		{
			name: "falha do parser na extracao value-only deve falhar o step",
			args: args{state: OnboardingState{UserID: "u1", Goal: "viajar", GoalValueAsked: true, ResumeText: "400 mil"}},
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
		{
			name: "json invalido na extracao value-only deve falhar o step",
			args: args{state: OnboardingState{UserID: "u1", Goal: "viajar", GoalValueAsked: true, ResumeText: "400 mil"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: []byte("nao-e-json")}, nil).Once()
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

func (s *OnboardingWorkflowSuite) TestBuildGoalStep_NoValueCombinationCompletesWithEmptyGoal() {
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
	}{
		{
			name: "valor invalido junto com objetivo vazio nunca completa sem objetivo",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "..."}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "", HasAmount: true, AmountBRL: 400000})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
		},
		{
			name: "valor negativo junto com objetivo vazio nunca completa sem objetivo",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "..."}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "", HasAmount: true, AmountBRL: -100})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
		},
		{
			name: "ausencia de valor junto com objetivo vazio nunca completa sem objetivo",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "..."}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(goalWithValueExtract{Goal: "", HasAmount: false, AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepGoalID, BuildGoalStep(scenario.dependencies.agentMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			s.NoError(err)
			s.NotEqual(workflow.StepStatusCompleted, out.Status, "nenhuma combinacao de valor deve completar o step com Goal vazio")
			s.Empty(out.State.Goal)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildIncomeStep() {
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
			name:         "primeira chamada deve perguntar a renda de forma deterministica",
			args:         args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{agentMock: s.agentMock},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(_incomePrompt, out.Suspend.Prompt)
				s.Equal(PhaseMonthlyIncome, out.State.Phase)
			},
		},
		{
			name: "resume com renda valida deve completar",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "R$ 13.500,00"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(incomeExtract{AmountBRL: 13500})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(int64(1350000), out.State.IncomeCents)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepIncomeID, BuildIncomeStep(scenario.dependencies.agentMock))
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
		agentMock   *agentmocks.Agent
		budgetsMock *interfacemocks.BudgetPlanner
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira chamada deve suspender com sugestao pre-preenchida em R$ e %",
			args: args{state: OnboardingState{UserID: "u1", IncomeCents: 1350000}},
			dependencies: dependencies{
				agentMock: s.agentMock,
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, _defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Contains(out.Suspend.Prompt, "Aceita esta sugestão")
				s.Contains(out.Suspend.Prompt, "R$")
				s.Contains(out.Suspend.Prompt, "40%")
				s.Contains(out.Suspend.Prompt, "💰 Custo Fixo")
			},
		},
		{
			name: "resume confirmando deve aplicar a distribuicao oficial",
			args: args{state: OnboardingState{UserID: "u1", IncomeCents: 1350000, ResumeText: "sim"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{Action: "confirm"})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, _defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(4000, out.State.Allocations["expense.custo_fixo"])
				s.Equal(3000, out.State.Allocations["expense.liberdade_financeira"])
			},
		},
		{
			name: "resume com valores em reais deve converter para basis points",
			args: args{state: OnboardingState{UserID: "u1", IncomeCents: 1350000, ResumeText: "custo 5400..."}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{
						Action: "reais", CustoFixo: 5400, Conhecimento: 1350,
						Prazeres: 1350, Metas: 1350, LiberdadeFinanceira: 4050,
					})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, _defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(4000, out.State.Allocations["expense.custo_fixo"])
			},
		},
		{
			name: "resume invalido deve re-suspender com mensagem deterministica sem 3a pessoa",
			args: args{state: OnboardingState{UserID: "u1", IncomeCents: 1350000, ResumeText: "40 10 10 10 20"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(allocationInputExtract{
						Action: "percent", CustoFixo: 40, Conhecimento: 10,
						Prazeres: 10, Metas: 10, LiberdadeFinanceira: 20,
					})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, _defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Contains(out.Suspend.Prompt, "90%")
				s.NotContains(out.Suspend.Prompt, "o usuário")
				s.NotContains(out.Suspend.Prompt, "você orienta")
				s.Empty(out.State.ResumeText)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepMethodologyID, BuildMethodologyStep(scenario.dependencies.agentMock, scenario.dependencies.budgetsMock))
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
		agentMock   *agentmocks.Agent
		budgetsMock *interfacemocks.BudgetPlanner
	}
	validState := OnboardingState{
		UserID: "u1", Goal: "economizar", IncomeCents: 1350000,
		Allocations: _defaultDistributionBP,
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira chamada deve suspender com resumo deterministico",
			args: args{state: validState},
			dependencies: dependencies{
				agentMock: s.agentMock,
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, _defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Contains(out.Suspend.Prompt, "revisar")
				s.Contains(out.Suspend.Prompt, "economizar")
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
				budgetsMock: s.budgetsMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume sem confirmacao deve re-suspender de forma deterministica",
			args: args{state: OnboardingState{UserID: "u1", Goal: "economizar", ResumeText: "nao"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(yesNoExtract{Confirmed: false})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: s.budgetsMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(_summaryReprompt, out.Suspend.Prompt)
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
				budgetsMock: s.budgetsMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepSummaryID, BuildSummaryStep(scenario.dependencies.agentMock, scenario.dependencies.budgetsMock))
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
		IncomeCents: 1350000,
		Allocations: _defaultDistributionBP,
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
						CreateBudget(mock.Anything, mock.MatchedBy(func(d interfaces.DraftBudget) bool {
							sum := 0
							for _, a := range d.Allocations {
								sum += a.BasisPoints
							}
							return sum == 10000
						})).
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

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.agentMock, s.budgetsMock, s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(_conclusionRecurrencePrompt, out.Suspend.Prompt)
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
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, map[string]any{"objetivo_financeiro": state.Goal}).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.agentMock, s.budgetsMock, s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.True(out.State.Recurrence)
	s.Contains(out.State.FinalMessage, "economizar")
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_WithGoalValuePersistsMetadataAndMessage() {
	state := OnboardingState{
		UserID:         "11111111-1111-1111-1111-111111111111",
		Goal:           "comprar uma casa",
		GoalValueCents: 40000000,
		ResumeText:     "sim",
	}
	payload, _ := json.Marshal(yesNoExtract{Confirmed: true})
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()
	s.budgetsMock.EXPECT().
		CreateRecurrence(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), 12).
		Return(nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, map[string]any{
			"objetivo_financeiro":                state.Goal,
			"objetivo_financeiro_valor_centavos": state.GoalValueCents,
		}).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.agentMock, s.budgetsMock, s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.FinalMessage, "comprar uma casa")
	s.Contains(out.State.FinalMessage, "meta de "+formatBRL(state.GoalValueCents))
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_WithoutGoalValueOmitsMetadataKeyAndMarkdownUnchanged() {
	state := OnboardingState{
		UserID:     "11111111-1111-1111-1111-111111111111",
		Goal:       "economizar",
		ResumeText: "nao",
	}
	payload, _ := json.Marshal(yesNoExtract{Confirmed: false})
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, map[string]any{"objetivo_financeiro": state.Goal}).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.agentMock, s.budgetsMock, s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.False(out.State.Recurrence)
	s.Contains(out.State.FinalMessage, "economizar")
	s.NotContains(out.State.FinalMessage, "meta de")
}

func (s *OnboardingWorkflowSuite) TestConclusionFinalMessage_WithValueMentionsAmount() {
	msg := conclusionFinalMessage("comprar uma casa", 40000000)
	s.Contains(msg, fmt.Sprintf(`Seu objetivo "comprar uma casa" (meta de %s)`, formatBRL(40000000)))
}

func (s *OnboardingWorkflowSuite) TestConclusionFinalMessage_WithoutValueMatchesPreviousBehavior() {
	msg := conclusionFinalMessage("economizar", 0)
	expected := fmt.Sprintf(
		"Tudo pronto! 🚀 %s está registrado.\n\n"+
			"Agora é só começar: me envie seus gastos e receitas no dia a dia (ex.: \"gastei R$ 50 no mercado\" ou \"recebi R$ 200 de freela\") que eu registro tudo pra você. Vamos juntos! 💪",
		`Seu objetivo "economizar"`,
	)
	s.Equal(expected, msg)
	s.NotContains(msg, "meta de")
}

func (s *OnboardingWorkflowSuite) TestBuildOnboardingWorkflow_IDAndStructure() {
	s.agentMock.EXPECT().ID().Return("onboarding-agent").Maybe()
	def := BuildOnboardingWorkflow(s.agentMock, s.cardsMock, s.budgetsMock, s.wmMock, s.threadsMock, s.messagesMock)
	s.Equal(OnboardingWorkflowID, def.ID)
	s.NotNil(def.Root)
	s.True(def.Durable)
	s.Equal(3, def.MaxAttempts)
}

func (s *OnboardingWorkflowSuite) TestWrapStepWithMessages_AppendsOutboundOnSuspend() {
	threadPK := uuid.New()
	s.threadsMock.EXPECT().
		GetOrCreate(mock.Anything, "user-x", "peer-x").
		Return(memory.Thread{ID: threadPK, ResourceID: "user-x", ThreadID: "peer-x"}, nil).
		Once()
	s.messagesMock.EXPECT().
		Append(mock.Anything, threadPK, mock.MatchedBy(func(m memory.Message) bool {
			return m.Role == memory.RoleAssistant && m.Content == "ola!"
		})).
		Return(nil).
		Once()

	inner := func(_ context.Context, st OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		return workflow.StepOutput[OnboardingState]{
			State:   st,
			Status:  workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: "ola!"},
		}, nil
	}
	wrapped := wrapStepWithMessages(inner, s.threadsMock, s.messagesMock)

	state := OnboardingState{UserID: "user-x", PeerID: "peer-x"}
	out, err := wrapped(s.ctx, state)
	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
}

func (s *OnboardingWorkflowSuite) TestWrapStepWithMessages_AppendsInboundOnResume() {
	threadPK := uuid.New()
	s.threadsMock.EXPECT().
		GetOrCreate(mock.Anything, "user-y", "peer-y").
		Return(memory.Thread{ID: threadPK, ResourceID: "user-y", ThreadID: "peer-y"}, nil).
		Once()
	s.messagesMock.EXPECT().
		Append(mock.Anything, threadPK, mock.MatchedBy(func(m memory.Message) bool {
			return m.Role == memory.RoleUser && m.Content == "resposta do usuario"
		})).
		Return(nil).
		Once()

	inner := func(_ context.Context, st OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		st.ResumeText = ""
		return workflow.StepOutput[OnboardingState]{State: st, Status: workflow.StepStatusCompleted}, nil
	}
	wrapped := wrapStepWithMessages(inner, s.threadsMock, s.messagesMock)

	state := OnboardingState{UserID: "user-y", PeerID: "peer-y", ResumeText: "resposta do usuario"}
	out, err := wrapped(s.ctx, state)
	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
}

func (s *OnboardingWorkflowSuite) TestWrapStepWithMessages_NoPeerID_NoAppend() {
	inner := func(_ context.Context, st OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		return workflow.StepOutput[OnboardingState]{
			State:   st,
			Status:  workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: "ola!"},
		}, nil
	}
	wrapped := wrapStepWithMessages(inner, s.threadsMock, s.messagesMock)

	state := OnboardingState{UserID: "user-z", PeerID: ""}
	out, err := wrapped(s.ctx, state)
	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
}

func (s *OnboardingWorkflowSuite) TestWelcomeGoalPromptHasNoThirdPersonLeak() {
	s.NotContains(_welcomeGoalPrompt, "o usuário")
	s.NotContains(_welcomeGoalPrompt, "peça")
	s.True(strings.Contains(_welcomeGoalPrompt, "Vamos começar?"))
}
