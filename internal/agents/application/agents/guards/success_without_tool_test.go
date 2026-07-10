package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type SuccessWithoutToolGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSuccessWithoutToolGuardSuite(t *testing.T) {
	suite.Run(t, new(SuccessWithoutToolGuardSuite))
}

func (s *SuccessWithoutToolGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SuccessWithoutToolGuardSuite) TestName() {
	guard := NewSuccessWithoutToolGuard()
	s.Equal("success_without_tool", guard.Name())
}

func (s *SuccessWithoutToolGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "marcador de sucesso sem nenhuma write-tool chamada -> fallback + Failed",
			args: args{out: agent.Result{Content: "Registrei sua despesa com sucesso!"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "marcador de sucesso com write-tool bem-sucedida -> nao trata",
			args: args{out: agent.Result{
				Content: "Registrei sua despesa com sucesso!",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"routed"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "marcador de sucesso mas write-tool falhou -> fallback + Failed",
			args: args{out: agent.Result{
				Content: "Registrei sua despesa com sucesso!",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeError, Content: "falha"},
				},
			}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
			},
		},
		{
			name: "resposta clarify verbatim contendo marcador nao trata (relay legitimo)",
			args: args{out: agent.Result{
				Content: "Confirma o registro do lançamento?",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"clarify","message":"Confirma o registro do lançamento?"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "sem marcador de sucesso -> nao trata",
			args: args{out: agent.Result{Content: "Como posso te ajudar hoje?"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewSuccessWithoutToolGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}
