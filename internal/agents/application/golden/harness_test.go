package golden

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type HarnessSuite struct {
	suite.Suite
	ctx context.Context
}

func TestHarnessSuite(t *testing.T) {
	suite.Run(t, new(HarnessSuite))
}

func (s *HarnessSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *HarnessSuite) TestEvaluateCase() {
	type args struct {
		executor AgentExecutor
		c        Case
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(outcome CaseOutcome)
	}{
		{
			name: "deve passar quando tool esperada foi chamada e resposta satisfaz propriedade",
			args: args{
				executor: func(_ context.Context, _ []llm.Message) (agent.Result, error) {
					return agent.Result{
						Content:   "registrei sua despesa",
						ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
					}, nil
				},
				c: Case{
					Name:             "caso ok",
					Category:         CategoryExpenseIncome,
					Input:            "gastei 10 reais",
					ExpectedTool:     "register_expense",
					ResponseProperty: nonEmptyResponse,
					ResponseDescribe: "resposta não vazia",
				},
			},
			expect: func(outcome CaseOutcome) {
				s.True(outcome.Passed)
			},
		},
		{
			name: "deve falhar quando tool esperada nao foi chamada",
			args: args{
				executor: func(_ context.Context, _ []llm.Message) (agent.Result, error) {
					return agent.Result{Content: "ok", ToolCalls: nil}, nil
				},
				c: Case{
					Name:             "caso sem tool",
					Category:         CategoryExpenseIncome,
					Input:            "gastei 10 reais",
					ExpectedTool:     "register_expense",
					ResponseProperty: nonEmptyResponse,
					ResponseDescribe: "resposta não vazia",
				},
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "register_expense")
			},
		},
		{
			name: "deve falhar quando todas as expectedTools nao foram chamadas",
			args: args{
				executor: func(_ context.Context, _ []llm.Message) (agent.Result, error) {
					return agent.Result{
						Content:   "ok",
						ToolCalls: []agent.ToolCallRecord{{Tool: "query_month", Outcome: agent.ToolCallOutcomeSuccess}},
					}, nil
				},
				c: Case{
					Name:             "caso multi tool parcial",
					Category:         CategoryQuery,
					Input:            "como estou indo?",
					ExpectedTools:    []string{"query_month", "query_plan"},
					ResponseProperty: nonEmptyResponse,
					ResponseDescribe: "resposta não vazia",
				},
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "query_plan")
			},
		},
		{
			name: "deve falhar quando tool foi chamada mas nenhuma era esperada",
			args: args{
				executor: func(_ context.Context, _ []llm.Message) (agent.Result, error) {
					return agent.Result{
						Content:   "ok",
						ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
					}, nil
				},
				c: Case{
					Name:             "caso multi item",
					Category:         CategoryExpenseIncome,
					Input:            "gastei 10 e 20 reais",
					NoToolExpected:   true,
					ResponseProperty: nonEmptyResponse,
					ResponseDescribe: "resposta não vazia",
				},
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
			},
		},
		{
			name: "deve falhar quando propriedade de resposta nao e satisfeita",
			args: args{
				executor: func(_ context.Context, _ []llm.Message) (agent.Result, error) {
					return agent.Result{
						Content:   "",
						ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
					}, nil
				},
				c: Case{
					Name:             "resposta vazia",
					Category:         CategoryExpenseIncome,
					Input:            "gastei 10 reais",
					ExpectedTool:     "register_expense",
					ResponseProperty: nonEmptyResponse,
					ResponseDescribe: "resposta não vazia",
				},
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "resposta não vazia")
			},
		},
		{
			name: "deve falhar quando executor retorna erro",
			args: args{
				executor: func(_ context.Context, _ []llm.Message) (agent.Result, error) {
					return agent.Result{}, errors.New("falha de transporte")
				},
				c: Case{
					Name:             "erro de execucao",
					Category:         CategoryExpenseIncome,
					Input:            "gastei 10 reais",
					ExpectedTool:     "register_expense",
					ResponseProperty: nonEmptyResponse,
					ResponseDescribe: "resposta não vazia",
				},
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "falha de transporte")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			outcome := EvaluateCase(s.ctx, scenario.args.executor, scenario.args.c)
			scenario.expect(outcome)
		})
	}
}

func (s *HarnessSuite) TestEvaluateCaseIncludesPriorTurns() {
	var capturedMessages []llm.Message
	executor := func(_ context.Context, messages []llm.Message) (agent.Result, error) {
		capturedMessages = messages
		return agent.Result{
			Content:   "ok",
			ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
		}, nil
	}
	c := Case{
		Name:     "caso com turno anterior",
		Category: CategoryPending,
		PriorTurns: []Turn{
			{UserMessage: "gastei 40 reais no mercado"},
		},
		Input:            "confirma pagamento débito",
		ExpectedTool:     "register_expense",
		ResponseProperty: nonEmptyResponse,
		ResponseDescribe: "resposta não vazia",
	}

	outcome := EvaluateCase(s.ctx, executor, c)

	s.True(outcome.Passed)
	s.Len(capturedMessages, 2)
	s.Equal("gastei 40 reais no mercado", capturedMessages[0].Content)
	s.Equal("confirma pagamento débito", capturedMessages[1].Content)
}

func (s *HarnessSuite) TestEvaluateCaseWithCaptureChecksExpectedArgs() {
	type args struct {
		captured []CapturedToolCall
		c        Case
	}

	executor := func(_ context.Context, _ []llm.Message) (agent.Result, error) {
		return agent.Result{
			Content:   "ok",
			ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
		}, nil
	}

	baseCase := Case{
		Name:         "caso com args esperados",
		Category:     CategoryExpenseIncome,
		Input:        "gastei 50 reais",
		ExpectedTool: "register_expense",
		ExpectedArgs: map[string]any{
			"amountCents": 5000.0,
		},
		ResponseProperty: nonEmptyResponse,
		ResponseDescribe: "resposta não vazia",
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(outcome CaseOutcome)
	}{
		{
			name: "deve passar quando arg numerico bate exatamente",
			args: args{
				captured: []CapturedToolCall{{Tool: "register_expense", Args: map[string]any{"amountCents": 5000.0}}},
				c:        baseCase,
			},
			expect: func(outcome CaseOutcome) {
				s.True(outcome.Passed)
			},
		},
		{
			name: "deve passar quando arg numerico vem como string parseavel",
			args: args{
				captured: []CapturedToolCall{{Tool: "register_expense", Args: map[string]any{"amountCents": "5000"}}},
				c:        baseCase,
			},
			expect: func(outcome CaseOutcome) {
				s.True(outcome.Passed)
			},
		},
		{
			name: "deve falhar quando arg numerico diverge",
			args: args{
				captured: []CapturedToolCall{{Tool: "register_expense", Args: map[string]any{"amountCents": 4000.0}}},
				c:        baseCase,
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "amountCents")
			},
		},
		{
			name: "deve falhar quando arg obrigatorio ausente",
			args: args{
				captured: []CapturedToolCall{{Tool: "register_expense", Args: map[string]any{"description": "mercado"}}},
				c:        baseCase,
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "ausente")
			},
		},
		{
			name: "deve falhar quando tool esperada nao foi capturada",
			args: args{
				captured: nil,
				c:        baseCase,
			},
			expect: func(outcome CaseOutcome) {
				s.False(outcome.Passed)
				s.Contains(outcome.Detail, "não foi capturada")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			outcome := EvaluateCaseWithCapture(s.ctx, executor, scenario.args.c, func() []CapturedToolCall { return scenario.args.captured })
			scenario.expect(outcome)
		})
	}
}
