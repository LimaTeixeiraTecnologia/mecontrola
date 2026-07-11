package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
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
		s.Contains(goalWithValueSystemPrompt, example)
		s.Contains(goalValueSystemPrompt, example)
	}
}

func (s *OnboardingWorkflowSuite) TestGoalValueReprompt_InvitesOptionalValueWithoutBlocking() {
	s.NotEmpty(goalValueReprompt)
	s.Contains(strings.ToLower(goalValueReprompt), "não")
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

func (s *OnboardingWorkflowSuite) TestOnboardingState_MergePatch_PreservesMonthlyBudgetAndReviewAwait() {
	codec := workflow.NewCodec[OnboardingState]()
	base := OnboardingState{
		Phase:              PhaseBudgetReview,
		UserID:             "user-1",
		Goal:               "comprar uma casa",
		MonthlyBudgetCents: 1350000,
		ReviewAwait:        reviewAwaitConfirm,
		Allocations:        map[string]int{"expense.custo_fixo": 4000},
	}

	baseBytes, err := codec.Encode(base)
	s.Require().NoError(err)

	patch := []byte(`{"resumeText":"sim"}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	s.Require().NoError(err)

	result, err := codec.Decode(merged)
	s.Require().NoError(err)

	s.Equal(int64(1350000), result.MonthlyBudgetCents)
	s.Equal(reviewAwaitConfirm, result.ReviewAwait)
	s.Equal(4000, result.Allocations["expense.custo_fixo"])
	s.Equal("sim", result.ResumeText)
}

func (s *OnboardingWorkflowSuite) TestDecideMonthlyBudgetCents() {
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
			cents, err := DecideMonthlyBudgetCents(scenario.args.amountBRL)
			scenario.expect(cents, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestCompetenceLocationFallsBackToUTC() {
	saoPaulo, loadErr := time.LoadLocation("America/Sao_Paulo")
	scenarios := []struct {
		name   string
		loc    *time.Location
		err    error
		expect func(loc *time.Location)
	}{
		{
			name:   "deve cair para UTC quando LoadLocation falha (runtime distroless sem tzdata)",
			loc:    nil,
			err:    errors.New("unknown time zone America/Sao_Paulo"),
			expect: func(loc *time.Location) { s.NotNil(loc); s.Equal(time.UTC, loc) },
		},
		{
			name:   "deve cair para UTC quando loc e nil sem erro",
			loc:    nil,
			err:    nil,
			expect: func(loc *time.Location) { s.NotNil(loc); s.Equal(time.UTC, loc) },
		},
		{
			name:   "deve preservar a localizacao carregada com sucesso",
			loc:    saoPaulo,
			err:    loadErr,
			expect: func(loc *time.Location) { s.NotNil(loc); s.Equal(saoPaulo, loc) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.NotPanics(func() {
				loc := competenceLocation(scenario.loc, scenario.err)
				_ = time.Now().In(loc).Format("2006-01")
				scenario.expect(loc)
			})
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
			name: "confirm sem valores deve retornar a distribuicao oficial",
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
			name: "confirm com valores nao-nulos nao aplica default e pede reprompt",
			args: args{kind: allocationInputConfirm, values: map[string]float64{
				"expense.custo_fixo": 2500, "expense.conhecimento": 0, "expense.prazeres": 500,
				"expense.metas": 0, "expense.liberdade_financeira": 2000,
			}, incomeCents: 500000},
			expect: func(bp map[string]int, err error) {
				s.Error(err)
				s.Nil(bp)
				s.ErrorIs(err, errAllocationConfirmWithValues)
			},
		},
		{
			name: "reais caso real preserva 5000/0/1000/0/4000 sobre renda 500000",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 2500, "expense.conhecimento": 0, "expense.prazeres": 500,
				"expense.metas": 0, "expense.liberdade_financeira": 2000,
			}, incomeCents: 500000},
			expect: func(bp map[string]int, err error) {
				s.NoError(err)
				s.Equal(5000, bp["expense.custo_fixo"])
				s.Equal(0, bp["expense.conhecimento"])
				s.Equal(1000, bp["expense.prazeres"])
				s.Equal(0, bp["expense.metas"])
				s.Equal(4000, bp["expense.liberdade_financeira"])
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
			name: "reais que nao somam ao orcamento mensal deve retornar erro amigavel",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 5400, "expense.conhecimento": 1350, "expense.prazeres": 1350,
				"expense.metas": 1350, "expense.liberdade_financeira": 2700,
			}, incomeCents: 1350000},
			expect: func(bp map[string]int, err error) {
				s.Error(err)
				s.Nil(bp)
				s.Contains(err.Error(), "orçamento mensal")
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

func (s *OnboardingWorkflowSuite) TestDecideAllocationKind() {
	type args struct {
		kind        allocationInputKind
		values      map[string]float64
		incomeCents int64
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(kind allocationInputKind)
	}{
		{
			name: "soma zero reclassifica para confirm",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 0, "expense.conhecimento": 0, "expense.prazeres": 0,
				"expense.metas": 0, "expense.liberdade_financeira": 0,
			}, incomeCents: 500000},
			expect: func(kind allocationInputKind) { s.Equal(allocationInputConfirm, kind) },
		},
		{
			name: "soma aproxima renda reclassifica confirm para reais (caso real)",
			args: args{kind: allocationInputConfirm, values: map[string]float64{
				"expense.custo_fixo": 2500, "expense.conhecimento": 0, "expense.prazeres": 500,
				"expense.metas": 0, "expense.liberdade_financeira": 2000,
			}, incomeCents: 500000},
			expect: func(kind allocationInputKind) { s.Equal(allocationInputReais, kind) },
		},
		{
			name: "soma aproxima 100 reclassifica para percent",
			args: args{kind: allocationInputConfirm, values: map[string]float64{
				"expense.custo_fixo": 40, "expense.conhecimento": 10, "expense.prazeres": 10,
				"expense.metas": 10, "expense.liberdade_financeira": 30,
			}, incomeCents: 500000},
			expect: func(kind allocationInputKind) { s.Equal(allocationInputPercent, kind) },
		},
		{
			name: "sem invariante numerica preserva classificacao do llm",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 300, "expense.conhecimento": 0, "expense.prazeres": 0,
				"expense.metas": 0, "expense.liberdade_financeira": 0,
			}, incomeCents: 500000},
			expect: func(kind allocationInputKind) { s.Equal(allocationInputReais, kind) },
		},
		{
			name: "reais legitimo permanece reais",
			args: args{kind: allocationInputReais, values: map[string]float64{
				"expense.custo_fixo": 5400, "expense.conhecimento": 1350, "expense.prazeres": 1350,
				"expense.metas": 1350, "expense.liberdade_financeira": 4050,
			}, incomeCents: 1350000},
			expect: func(kind allocationInputKind) { s.Equal(allocationInputReais, kind) },
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			kind := DecideAllocationKind(scenario.args.kind, scenario.args.values, scenario.args.incomeCents)
			scenario.expect(kind)
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
			name:   "deve parsear goal",
			args:   args{s: "goal"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseGoal, phase) },
		},
		{
			name:   "deve parsear monthly_budget",
			args:   args{s: "monthly_budget"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseMonthlyBudget, phase) },
		},
		{
			name:   "deve parsear budget_review",
			args:   args{s: "budget_review"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseBudgetReview, phase) },
		},
		{
			name:   "deve parsear activation",
			args:   args{s: "activation"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseActivation, phase) },
		},
		{
			name:   "deve parsear recurrence",
			args:   args{s: "recurrence"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseRecurrence, phase) },
		},
		{
			name:   "deve parsear cards",
			args:   args{s: "cards"},
			expect: func(phase OnboardingPhase, err error) { s.NoError(err); s.Equal(PhaseCards, phase) },
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
		{
			name:   "deve retornar erro para fase antiga removida",
			args:   args{s: "monthly_income"},
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

func (s *OnboardingWorkflowSuite) TestOnboardingPhase_String_RoundTrip() {
	phases := []OnboardingPhase{
		PhaseWelcome, PhaseGoal, PhaseMonthlyBudget, PhaseBudgetReview,
		PhaseActivation, PhaseRecurrence, PhaseCards, PhaseConclusion,
	}
	for _, phase := range phases {
		s.Run(phase.String(), func() {
			s.True(phase.IsValid())
			parsed, err := ParseOnboardingPhase(phase.String())
			s.NoError(err)
			s.Equal(phase, parsed)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestOnboardingPhase_IsValid_ZeroValue() {
	var zero OnboardingPhase
	s.False(zero.IsValid())
	s.Equal("unknown", zero.String())
}

func (s *OnboardingWorkflowSuite) TestReviewAwaitKind_IsValid_ZeroValue() {
	var zero reviewAwaitKind
	s.False(zero.IsValid())
	s.Equal("unknown", zero.String())
	s.True(reviewAwaitDistribution.IsValid())
	s.True(reviewAwaitConfirm.IsValid())
}

func (s *OnboardingWorkflowSuite) TestBuildWelcomeStep() {
	type args struct {
		state OnboardingState
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira entrada deve suspender com boas-vindas isolada sem meta orcamento renda ou cartao",
			args: args{state: OnboardingState{UserID: "u1"}},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(welcomePrompt, out.Suspend.Prompt)
				s.NotContains(out.Suspend.Prompt, "?")
				s.NotContains(out.Suspend.Prompt, "orçamento")
				s.NotContains(out.Suspend.Prompt, "renda")
				s.NotContains(out.Suspend.Prompt, "cartão")
				s.Equal(PhaseWelcome, out.State.Phase)
			},
		},
		{
			name: "resume com qualquer texto deve completar ignorando o conteudo (D-07)",
			args: args{state: OnboardingState{UserID: "u1", Phase: PhaseWelcome, ResumeText: "texto irrelevante que nao deve virar objetivo"}},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Empty(out.State.ResumeText)
				s.Empty(out.State.Goal)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(stepWelcomeID, BuildWelcomeStep())
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
			name:         "primeira mensagem deve perguntar o objetivo sem preambulo de boas-vindas",
			args:         args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{agentMock: s.agentMock},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(welcomeGoalPrompt, out.Suspend.Prompt)
				s.Contains(out.Suspend.Prompt, "Vamos começar?")
				s.Contains(out.Suspend.Prompt, "objetivo")
				s.Contains(out.Suspend.Prompt, "valor da meta")
				s.Contains(out.Suspend.Prompt, "R$ 400.000,00")
				s.NotContains(out.Suspend.Prompt, "Bem-vindo")
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
				s.Equal(goalValueReprompt, out.Suspend.Prompt)
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
				s.Equal(goalReprompt, out.Suspend.Prompt)
				s.Empty(out.State.Goal)
				s.False(out.State.GoalValueAsked)
			},
		},
		{
			name: "resume da repergunta de objetivo com meta valida sem valor deve honrar a pergunta opcional de valor",
			args: args{state: OnboardingState{UserID: "u1", GoalValueAsked: false, ResumeText: "quero viajar"}},
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
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(goalValueReprompt, out.Suspend.Prompt)
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
				s.Equal(goalValueReprompt, out.Suspend.Prompt)
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
				s.Equal(goalReprompt, out.Suspend.Prompt)
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

func (s *OnboardingWorkflowSuite) TestBuildMonthlyBudgetStep() {
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
			name:         "primeira chamada deve apresentar as 5 categorias e perguntar o orcamento mensal em mensagem unica",
			args:         args{state: OnboardingState{UserID: "u1"}},
			dependencies: dependencies{agentMock: s.agentMock},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(monthlyBudgetPrompt, out.Suspend.Prompt)
				s.Contains(out.Suspend.Prompt, "Antes de montar seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui. Tudo vive em apenas 5 categorias: Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira.")
				s.Contains(out.Suspend.Prompt, "orçamento mensal")
				s.NotContains(out.Suspend.Prompt, "renda")
				s.NotContains(out.Suspend.Prompt, "Faz sentido?")
				s.Equal(PhaseMonthlyBudget, out.State.Phase)
			},
		},
		{
			name: "resume com orcamento valido deve completar e persistir MonthlyBudgetCents",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "R$ 13.500,00"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(monthlyBudgetExtract{AmountBRL: 13500})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(int64(1350000), out.State.MonthlyBudgetCents)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume sem valor positivo identificavel deve reperguntar com exemplo em reais sem criar nada",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "nao sei"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(monthlyBudgetExtract{AmountBRL: 0})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(monthlyBudgetReprompt, out.Suspend.Prompt)
				s.Contains(out.Suspend.Prompt, "R$ 3.500,00")
				s.Equal(int64(0), out.State.MonthlyBudgetCents)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "falha do parser deve falhar o step",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "R$ 3.000"}},
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
			name: "json invalido deve falhar o step",
			args: args{state: OnboardingState{UserID: "u1", ResumeText: "R$ 3.000"}},
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
			step := workflow.NewStepFunc(stepMonthlyBudgetID, BuildMonthlyBudgetStep(scenario.dependencies.agentMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildBudgetReviewStep() {
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
			name: "primeira chamada deve suspender com sugestao pre-preenchida em R$ e % e ReviewAwait=distribution",
			args: args{state: OnboardingState{UserID: "u1", MonthlyBudgetCents: 1350000}},
			dependencies: dependencies{
				agentMock: s.agentMock,
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, defaultDistributionBP), nil).Once()
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
				s.Equal(reviewAwaitDistribution, out.State.ReviewAwait)
				s.Equal(PhaseBudgetReview, out.State.Phase)
			},
		},
		{
			name: "resume em reviewAwaitDistribution confirmando deve aplicar distribuicao oficial, recriar draft e suspender no resumo",
			args: args{state: OnboardingState{
				UserID: "11111111-1111-1111-1111-111111111111", Goal: "economizar",
				MonthlyBudgetCents: 1350000, ResumeText: "sim", ReviewAwait: reviewAwaitDistribution,
			}},
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
						Return(suggestReturn(1350000, defaultDistributionBP), nil).Twice()
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
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Contains(out.Suspend.Prompt, "revisar a distribuição")
				s.Contains(out.Suspend.Prompt, "economizar")
				s.Equal(4000, out.State.Allocations["expense.custo_fixo"])
				s.Equal(3000, out.State.Allocations["expense.liberdade_financeira"])
				s.Equal(reviewAwaitConfirm, out.State.ReviewAwait)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume em reviewAwaitDistribution com valores em reais deve converter para basis points e recriar draft existente",
			args: args{state: OnboardingState{
				UserID: "11111111-1111-1111-1111-111111111111", Goal: "economizar",
				MonthlyBudgetCents: 1350000, ResumeText: "custo 5400...", ReviewAwait: reviewAwaitDistribution,
			}},
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
						Return(suggestReturn(1350000, defaultDistributionBP), nil).Twice()
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
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(4000, out.State.Allocations["expense.custo_fixo"])
				s.Equal(reviewAwaitConfirm, out.State.ReviewAwait)
			},
		},
		{
			name: "resume em reviewAwaitDistribution com soma que nao fecha deve reprompt sem ativar, mantendo mesmo sub-estado",
			args: args{state: OnboardingState{
				UserID: "u1", MonthlyBudgetCents: 1350000, ResumeText: "40 10 10 10 20", ReviewAwait: reviewAwaitDistribution,
			}},
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
						Return(suggestReturn(1350000, defaultDistributionBP), nil).Once()
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
				s.Equal(reviewAwaitDistribution, out.State.ReviewAwait)
			},
		},
		{
			name: "resume em reviewAwaitConfirm com sim deve completar e avancar para activation",
			args: args{state: OnboardingState{
				UserID: "u1", Goal: "economizar", ResumeText: "sim, confirmo", ReviewAwait: reviewAwaitConfirm,
			}},
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
			name: "resume em reviewAwaitConfirm com nao deve reabrir distribuicao (D-09) sem ativar parcial",
			args: args{state: OnboardingState{
				UserID: "u1", Goal: "economizar", MonthlyBudgetCents: 1350000,
				ResumeText: "nao", ReviewAwait: reviewAwaitConfirm,
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(yesNoExtract{Confirmed: false})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Contains(out.Suspend.Prompt, "Aceita esta sugestão")
				s.Equal(reviewAwaitDistribution, out.State.ReviewAwait)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resume em reviewAwaitConfirm ambiguo deve reabrir distribuicao (D-09)",
			args: args{state: OnboardingState{
				UserID: "u1", Goal: "economizar", MonthlyBudgetCents: 1350000,
				ResumeText: "talvez", ReviewAwait: reviewAwaitConfirm,
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					payload, _ := json.Marshal(yesNoExtract{Confirmed: false})
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: payload}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						SuggestAllocation(mock.Anything, int64(1350000), mock.Anything).
						Return(suggestReturn(1350000, defaultDistributionBP), nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.Equal(reviewAwaitDistribution, out.State.ReviewAwait)
			},
		},
		{
			name: "deve falhar quando execute retorna erro em reviewAwaitConfirm",
			args: args{state: OnboardingState{UserID: "u1", Goal: "economizar", ResumeText: "sim", ReviewAwait: reviewAwaitConfirm}},
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
			step := workflow.NewStepFunc(stepBudgetReviewID, BuildBudgetReviewStep(scenario.dependencies.agentMock, scenario.dependencies.budgetsMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *OnboardingWorkflowSuite) TestBuildActivationStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		budgetsMock *interfacemocks.BudgetPlanner
	}
	baseState := OnboardingState{
		UserID:             "11111111-1111-1111-1111-111111111111",
		MonthlyBudgetCents: 1350000,
		Allocations:        defaultDistributionBP,
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "deve ativar o orcamento e completar",
			args: args{state: baseState},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						ActivateBudget(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal(PhaseActivation, out.State.Phase)
			},
		},
		{
			name: "deve ser idempotente quando o orcamento ja esta ativo",
			args: args{state: baseState},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						ActivateBudget(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(interfaces.ErrBudgetAlreadyActive).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
			},
		},
		{
			name: "deve falhar quando activate_budget retorna erro diferente de ja-ativo",
			args: args{state: baseState},
			dependencies: dependencies{
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						ActivateBudget(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
						Return(errors.New("db down")).Once()
					return s.budgetsMock
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
			step := workflow.NewStepFunc(stepActivationID, BuildActivationStep(scenario.dependencies.budgetsMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func recurrenceExtractJSON(confirmed bool) []byte {
	b, _ := json.Marshal(yesNoExtract{Confirmed: confirmed})
	return b
}

func (s *OnboardingWorkflowSuite) TestBuildRecurrenceStep() {
	type args struct {
		state OnboardingState
	}
	type dependencies struct {
		agentMock   *agentmocks.Agent
		budgetsMock *interfacemocks.BudgetPlanner
	}
	baseState := OnboardingState{
		UserID:             "11111111-1111-1111-1111-111111111111",
		Phase:              PhaseActivation,
		MonthlyBudgetCents: 1350000,
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[OnboardingState], err error)
	}{
		{
			name: "primeira entrada deve suspender perguntando sobre recorrencia",
			args: args{state: baseState},
			dependencies: dependencies{
				agentMock:   s.agentMock,
				budgetsMock: s.budgetsMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(conclusionRecurrencePrompt, out.Suspend.Prompt)
				s.Equal(PhaseRecurrence, out.State.Phase)
			},
		},
		{
			name: "resposta afirmativa deve criar recorrencia de 12 meses e completar",
			args: args{state: OnboardingState{
				UserID:             baseState.UserID,
				Phase:              PhaseRecurrence,
				MonthlyBudgetCents: baseState.MonthlyBudgetCents,
				ResumeText:         "sim, quero repetir",
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: recurrenceExtractJSON(true)}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						CreateRecurrence(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), 12).
						Return(nil).Once()
					return s.budgetsMock
				}(),
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.True(out.State.Recurrence)
				s.Empty(out.State.ResumeText)
			},
		},
		{
			name: "resposta negativa nao deve criar recorrencia nem desfazer orcamento",
			args: args{state: OnboardingState{
				UserID:             baseState.UserID,
				Phase:              PhaseRecurrence,
				MonthlyBudgetCents: baseState.MonthlyBudgetCents,
				ResumeText:         "não, obrigado",
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: recurrenceExtractJSON(false)}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: s.budgetsMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.False(out.State.Recurrence)
			},
		},
		{
			name: "resposta ambigua deve seguir sem recorrencia e sem reprompt",
			args: args{state: OnboardingState{
				UserID:             baseState.UserID,
				Phase:              PhaseRecurrence,
				MonthlyBudgetCents: baseState.MonthlyBudgetCents,
				ResumeText:         "sei lá, talvez",
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: recurrenceExtractJSON(false)}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: s.budgetsMock,
			},
			expect: func(out workflow.StepOutput[OnboardingState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.False(out.State.Recurrence)
				s.NotEqual(workflow.StepStatusSuspended, out.Status)
			},
		},
		{
			name: "erro ao criar recorrencia deve falhar sem desfazer orcamento",
			args: args{state: OnboardingState{
				UserID:             baseState.UserID,
				Phase:              PhaseRecurrence,
				MonthlyBudgetCents: baseState.MonthlyBudgetCents,
				ResumeText:         "sim",
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().
						Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(agentpkg.Result{RawJSON: recurrenceExtractJSON(true)}, nil).Once()
					return s.agentMock
				}(),
				budgetsMock: func() *interfacemocks.BudgetPlanner {
					s.budgetsMock.EXPECT().
						CreateRecurrence(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), 12).
						Return(errors.New("db down")).Once()
					return s.budgetsMock
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
			step := workflow.NewStepFunc(stepRecurrenceID, BuildRecurrenceStep(scenario.dependencies.agentMock, scenario.dependencies.budgetsMock))
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func cardExtractJSON(t *testing.T, extract cardExtract) []byte {
	t.Helper()
	b, err := json.Marshal(extract)
	if err != nil {
		t.Fatalf("marshal card extract: %v", err)
	}
	return b
}

func (s *OnboardingWorkflowSuite) TestBuildCardsStep() {
	userID := "11111111-1111-1111-1111-111111111111"
	userUUID := uuid.MustParse(userID)

	s.Run("primeira entrada deve listar cartoes e suspender com o prompt", func() {
		s.SetupTest()
		s.cardsMock.EXPECT().ListCards(mock.Anything, userUUID).Return(nil, nil).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		out, err := step.Execute(s.ctx, OnboardingState{UserID: userID})

		s.NoError(err)
		s.Equal(workflow.StepStatusSuspended, out.Status)
		s.NotNil(out.Suspend)
		s.Equal(cardsPrompt(0), out.Suspend.Prompt)
		s.Equal(PhaseCards, out.State.Phase)
	})

	s.Run("erro ao listar cartoes deve falhar o step", func() {
		s.SetupTest()
		s.cardsMock.EXPECT().ListCards(mock.Anything, userUUID).Return(nil, errors.New("db down")).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		out, err := step.Execute(s.ctx, OnboardingState{UserID: userID})

		s.Error(err)
		s.Equal(workflow.StepStatusFailed, out.Status)
	})

	s.Run("recusa imediata deve marcar CardsDone e completar sem chamar CreateCard", func() {
		s.SetupTest()
		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: false})}, nil).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		out, err := step.Execute(s.ctx, OnboardingState{UserID: userID, Phase: PhaseCards, ResumeText: "não, obrigado"})

		s.NoError(err)
		s.Equal(workflow.StepStatusCompleted, out.Status)
		s.True(out.State.CardsDone)
		s.cardsMock.AssertNotCalled(s.T(), "CreateCard", mock.Anything, mock.Anything)
	})

	s.Run("cartao valido deve criar e re-suspender perguntando por outro (loop)", func() {
		s.SetupTest()
		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: true, Nickname: "Nubank", Bank: "Nubank", DueDay: 10})}, nil).Once()
		s.cardsMock.EXPECT().
			CreateCard(mock.Anything, interfaces.NewCard{UserID: userUUID, Nickname: "Nubank", Bank: "Nubank", DueDay: 10}).
			Return(interfaces.CardRef{}, nil).Once()
		s.cardsMock.EXPECT().
			ListCards(mock.Anything, userUUID).
			Return([]interfaces.Card{{Nickname: "Nubank"}}, nil).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		out, err := step.Execute(s.ctx, OnboardingState{UserID: userID, Phase: PhaseCards, ResumeText: "Nubank, vencimento dia 10"})

		s.NoError(err)
		s.Equal(workflow.StepStatusSuspended, out.Status)
		s.NotNil(out.Suspend)
		s.Equal(cardsPrompt(1), out.Suspend.Prompt)
		s.False(out.State.CardsDone)
	})

	s.Run("dois cartoes em turnos consecutivos devem criar ambos e encerrar apenas na recusa", func() {
		s.SetupTest()
		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: true, Nickname: "Nubank", Bank: "Nubank", DueDay: 10})}, nil).Once()
		s.cardsMock.EXPECT().
			CreateCard(mock.Anything, interfaces.NewCard{UserID: userUUID, Nickname: "Nubank", Bank: "Nubank", DueDay: 10}).
			Return(interfaces.CardRef{}, nil).Once()
		s.cardsMock.EXPECT().
			ListCards(mock.Anything, userUUID).
			Return([]interfaces.Card{{Nickname: "Nubank"}}, nil).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		firstOut, err := step.Execute(s.ctx, OnboardingState{UserID: userID, Phase: PhaseCards, ResumeText: "Nubank, vencimento dia 10"})
		s.NoError(err)
		s.Equal(workflow.StepStatusSuspended, firstOut.Status)
		s.Equal(cardsPrompt(1), firstOut.Suspend.Prompt)

		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: true, Nickname: "Inter", Bank: "Inter", DueDay: 5})}, nil).Once()
		s.cardsMock.EXPECT().
			CreateCard(mock.Anything, interfaces.NewCard{UserID: userUUID, Nickname: "Inter", Bank: "Inter", DueDay: 5}).
			Return(interfaces.CardRef{}, nil).Once()
		s.cardsMock.EXPECT().
			ListCards(mock.Anything, userUUID).
			Return([]interfaces.Card{{Nickname: "Nubank"}, {Nickname: "Inter"}}, nil).Once()

		secondState := firstOut.State
		secondState.ResumeText = "Inter, vencimento dia 5"
		secondOut, err := step.Execute(s.ctx, secondState)
		s.NoError(err)
		s.Equal(workflow.StepStatusSuspended, secondOut.Status)
		s.Equal(cardsPrompt(2), secondOut.Suspend.Prompt)

		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: false})}, nil).Once()

		thirdState := secondOut.State
		thirdState.ResumeText = "não, é só isso"
		thirdOut, err := step.Execute(s.ctx, thirdState)
		s.NoError(err)
		s.Equal(workflow.StepStatusCompleted, thirdOut.Status)
		s.True(thirdOut.State.CardsDone)
	})

	s.Run("cartao invalido deve re-suspender com reprompt sem criar cartao parcial", func() {
		s.SetupTest()
		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: true, Nickname: "", Bank: "Nubank", DueDay: 10})}, nil).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		out, err := step.Execute(s.ctx, OnboardingState{UserID: userID, Phase: PhaseCards, ResumeText: "vencimento dia 10, banco Nubank"})

		s.NoError(err)
		s.Equal(workflow.StepStatusSuspended, out.Status)
		s.Equal(cardsReprompt, out.Suspend.Prompt)
		s.False(out.State.CardsDone)
		s.cardsMock.AssertNotCalled(s.T(), "CreateCard", mock.Anything, mock.Anything)
	})

	s.Run("erro ao criar cartao deve falhar o step sem marcar CardsDone", func() {
		s.SetupTest()
		s.agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: cardExtractJSON(s.T(), cardExtract{WantsCard: true, Nickname: "Nubank", Bank: "Nubank", DueDay: 10})}, nil).Once()
		s.cardsMock.EXPECT().
			CreateCard(mock.Anything, interfaces.NewCard{UserID: userUUID, Nickname: "Nubank", Bank: "Nubank", DueDay: 10}).
			Return(interfaces.CardRef{}, errors.New("db down")).Once()

		step := workflow.NewStepFunc(stepCardsID, BuildCardsStep(s.agentMock, s.cardsMock))
		out, err := step.Execute(s.ctx, OnboardingState{UserID: userID, Phase: PhaseCards, ResumeText: "Nubank, vencimento dia 10"})

		s.Error(err)
		s.Equal(workflow.StepStatusFailed, out.Status)
		s.False(out.State.CardsDone)
	})
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_UpsertsWorkingMemoryAndSetsPhase() {
	state := OnboardingState{UserID: "11111111-1111-1111-1111-111111111111", Goal: "economizar", CardsDone: true}
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, map[string]any{"objetivo_financeiro": state.Goal}).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.True(out.State.CardsDone)
	s.Equal(PhaseConclusion, out.State.Phase)
	s.Contains(out.State.FinalMessage, "economizar")
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_WithGoalValuePersistsMetadataAndMessage() {
	state := OnboardingState{
		UserID:         "11111111-1111-1111-1111-111111111111",
		Goal:           "comprar uma casa",
		GoalValueCents: 40000000,
	}
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, map[string]any{
			"objetivo_financeiro":                state.Goal,
			"objetivo_financeiro_valor_centavos": state.GoalValueCents,
		}).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.FinalMessage, "comprar uma casa")
	s.Contains(out.State.FinalMessage, "meta de "+money.FromCents(state.GoalValueCents).BRL())
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_WithoutGoalValueOmitsMetadataKeyAndMarkdownUnchanged() {
	state := OnboardingState{
		UserID: "11111111-1111-1111-1111-111111111111",
		Goal:   "economizar",
	}
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, map[string]any{"objetivo_financeiro": state.Goal}).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.FinalMessage, "economizar")
	s.NotContains(out.State.FinalMessage, "meta de")
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_DoesNotReopenDistributionSummaryOrActivation() {
	state := OnboardingState{
		UserID:      "11111111-1111-1111-1111-111111111111",
		Goal:        "economizar",
		Allocations: defaultDistributionBP,
		Recurrence:  true,
	}
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, mock.AnythingOfType("string")).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, mock.AnythingOfType("map[string]interface {}")).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.wmMock))
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(defaultDistributionBP, out.State.Allocations)
	s.True(out.State.Recurrence)
}

func (s *OnboardingWorkflowSuite) TestBuildConclusionStep_WorkingMemoryHasNoIncomeLine() {
	state := OnboardingState{UserID: "11111111-1111-1111-1111-111111111111", Goal: "economizar"}
	var capturedContent string
	s.wmMock.EXPECT().
		Upsert(mock.Anything, state.UserID, mock.AnythingOfType("string")).
		Run(func(_ context.Context, _ string, content string) { capturedContent = content }).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, state.UserID, mock.AnythingOfType("map[string]interface {}")).
		Return(nil).Once()

	step := workflow.NewStepFunc(stepConclusionID, BuildConclusionStep(s.wmMock))
	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.NotContains(strings.ToLower(capturedContent), "renda")
	s.NotContains(strings.ToLower(capturedContent), "orçamento")
}

func (s *OnboardingWorkflowSuite) TestConclusionFinalMessage_WithValueMentionsAmount() {
	msg := conclusionFinalMessage("comprar uma casa", 40000000)
	s.Contains(msg, fmt.Sprintf(`Seu objetivo "comprar uma casa" (meta de %s)`, money.FromCents(40000000).BRL()))
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

func (s *OnboardingWorkflowSuite) TestBuildOnboardingWorkflow_SequenceStartsAtWelcomeAndSuspendsFirstEntry() {
	s.threadsMock.EXPECT().GetOrCreate(mock.Anything, mock.Anything, mock.Anything).
		Return(memory.Thread{ID: uuid.New()}, nil).Maybe()
	s.messagesMock.EXPECT().Append(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	def := BuildOnboardingWorkflow(s.agentMock, s.cardsMock, s.budgetsMock, s.wmMock, s.threadsMock, s.messagesMock)
	out, err := def.Root.Execute(s.ctx, OnboardingState{UserID: "user-x", PeerID: "peer-x"})

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(PhaseWelcome, out.State.Phase)
	s.NotNil(out.Suspend)
	s.Equal(welcomePrompt, out.Suspend.Prompt)
}

func (s *OnboardingWorkflowSuite) TestBuildOnboardingWorkflow_SequenceAdvancesWelcomeToGoalOnResume() {
	s.threadsMock.EXPECT().GetOrCreate(mock.Anything, mock.Anything, mock.Anything).
		Return(memory.Thread{ID: uuid.New()}, nil).Maybe()
	s.messagesMock.EXPECT().Append(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	def := BuildOnboardingWorkflow(s.agentMock, s.cardsMock, s.budgetsMock, s.wmMock, s.threadsMock, s.messagesMock)
	out, err := def.Root.Execute(s.ctx, OnboardingState{UserID: "user-x", PeerID: "peer-x", ResumeText: "vamos comecar"})

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(PhaseGoal, out.State.Phase)
	s.Equal("", out.State.ResumeText)
}

func (s *OnboardingWorkflowSuite) TestBuildOnboardingWorkflow_ActivationFailureProducesFailedStatusWithoutFalseSuccess() {
	s.threadsMock.EXPECT().GetOrCreate(mock.Anything, mock.Anything, mock.Anything).
		Return(memory.Thread{ID: uuid.New()}, nil).Maybe()
	s.messagesMock.EXPECT().Append(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.budgetsMock.EXPECT().
		ActivateBudget(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
		Return(errors.New("db down")).Once()

	state := OnboardingState{
		Phase:              PhaseActivation,
		UserID:             "11111111-1111-1111-1111-111111111111",
		PeerID:             "peer-x",
		MonthlyBudgetCents: 1350000,
		ReviewAwait:        0,
		Allocations:        defaultDistributionBP,
	}
	out, err := workflow.NewStepFunc(stepActivationID, wrapStepWithMessages(BuildActivationStep(s.budgetsMock), s.threadsMock, s.messagesMock)).Execute(s.ctx, state)

	s.Error(err)
	s.Equal(workflow.StepStatusFailed, out.Status)
	s.Empty(out.State.FinalMessage)
}

func (s *OnboardingWorkflowSuite) TestBuildOnboardingReaper_UsesOnboardingWorkflowIDAndTTL() {
	reaper := BuildOnboardingReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
	s.Equal(7*24*3600, int(OnboardingStaleAfter.Seconds()))
	s.Equal(100, OnboardingReaperBatch)
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
	s.NotContains(welcomeGoalPrompt, "o usuário")
	s.NotContains(welcomeGoalPrompt, "peça")
	s.True(strings.Contains(welcomeGoalPrompt, "Vamos começar?"))
}

func (s *OnboardingWorkflowSuite) TestM02_NoRendaTermInAnyOnboardingSurface() {
	rendaPattern := regexp.MustCompile(`(?i)\brenda\b`)

	sampleAllocations := suggestReturn(500000, defaultDistributionBP)
	sampleState := OnboardingState{
		UserID:             "user-m02",
		Goal:               "juntar uma reserva",
		MonthlyBudgetCents: 500000,
	}

	surfaces := map[string]string{
		"welcomePrompt":                   welcomePrompt,
		"welcomeGoalPrompt":               welcomeGoalPrompt,
		"goalReprompt":                    goalReprompt,
		"goalValueReprompt":               goalValueReprompt,
		"monthlyBudgetPrompt":             monthlyBudgetPrompt,
		"monthlyBudgetReprompt":           monthlyBudgetReprompt,
		"cardsReprompt":                   cardsReprompt,
		"conclusionRecurrencePrompt":      conclusionRecurrencePrompt,
		"allocationInputSystemPrompt":     allocationInputSystemPrompt,
		"summaryConfirmSystemPrompt":      summaryConfirmSystemPrompt,
		"goalWithValueSystemPrompt":       goalWithValueSystemPrompt,
		"goalValueSystemPrompt":           goalValueSystemPrompt,
		"monthlyBudgetSystemPrompt":       monthlyBudgetSystemPrompt,
		"cardsSystemPrompt":               cardsSystemPrompt,
		"recurrenceSystemPrompt":          recurrenceSystemPrompt,
		"cardsPrompt(0)":                  cardsPrompt(0),
		"cardsPrompt(2)":                  cardsPrompt(2),
		"methodologyPrompt":               methodologyPrompt(sampleAllocations),
		"methodologyReprompt":             methodologyReprompt("valores não fecham", sampleAllocations),
		"summaryPrompt":                   summaryPrompt(sampleState, sampleAllocations),
		"conclusionFinalMessage_valor":    conclusionFinalMessage("juntar reserva", 100000),
		"conclusionFinalMessage_semValor": conclusionFinalMessage("juntar reserva", 0),
		"renderAllocationLines":           renderAllocationLines(sampleAllocations),
	}

	for label, text := range surfaces {
		s.Falsef(rendaPattern.MatchString(text), "surface %s contém termo 'renda': %q", label, text)
	}

	decideErrors := []error{
		func() error { _, err := DecideGoal(""); return err }(),
		func() error { _, err := DecideMonthlyBudgetCents(0); return err }(),
		DecideDistribution(map[string]int{"expense.custo_fixo": 10000}),
		func() error {
			_, err := DecideAllocationsBP(allocationInputPercent, map[string]float64{"expense.custo_fixo": -10}, 500000)
			return err
		}(),
		func() error {
			_, err := DecideAllocationsBP(allocationInputPercent, map[string]float64{"expense.custo_fixo": 50}, 500000)
			return err
		}(),
		func() error {
			_, err := DecideAllocationsBP(allocationInputReais, map[string]float64{"expense.custo_fixo": -10}, 500000)
			return err
		}(),
		func() error {
			_, err := DecideAllocationsBP(allocationInputReais, map[string]float64{"expense.custo_fixo": 100}, 500000)
			return err
		}(),
		DecideCardEntry("", "", 0),
		DecideCardEntry("Nubank", "Nubank", 40),
		errInvalidOnboardingPhase,
		errInvalidReviewAwaitKind,
		errInvalidAllocationInput,
		errAllocationConfirmWithValues,
	}

	for i, err := range decideErrors {
		if err == nil {
			continue
		}
		s.Falsef(rendaPattern.MatchString(err.Error()), "decide error[%d] contém termo 'renda': %q", i, err.Error())
	}
}
