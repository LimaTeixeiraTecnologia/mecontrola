package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type RegistrationFailureWithoutToolGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRegistrationFailureWithoutToolGuardSuite(t *testing.T) {
	suite.Run(t, new(RegistrationFailureWithoutToolGuardSuite))
}

func (s *RegistrationFailureWithoutToolGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RegistrationFailureWithoutToolGuardSuite) TestName() {
	guard := NewRegistrationFailureWithoutToolGuard()
	s.Equal("registration_failure_without_tool", guard.Name())
}

func (s *RegistrationFailureWithoutToolGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "incidente 2026-07-30: LLM autorreporta falha de registro sem nenhuma tool call -> marca run como falho",
			args: args{out: agent.Result{Content: successWithoutToolFallbackMessage}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.False(decision.Retryable)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
				s.Equal("guard registration_failure_without_tool: LLM reportou falha de registro sem nenhuma chamada de ferramenta", decision.Result.FailureReason)
			},
		},
		{
			name: "fallback com tool call presente -> nao trata (outro guard ja decidiu)",
			args: args{out: agent.Result{
				Content: successWithoutToolFallbackMessage,
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "update_recurrence", Outcome: agent.ToolCallOutcomeError, Content: "falha"},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "resultado ja marcado como UsecaseError por outro guard -> nao duplica",
			args: args{out: agent.Result{
				Content:       successWithoutToolFallbackMessage,
				ToolOutcome:   agent.ToolOutcomeUsecaseError,
				FailureReason: "guard success_without_tool: confirmação de escrita fabricada pelo LLM sem write tool bem-sucedida",
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "conteudo diferente do fallback literal -> nao trata",
			args: args{out: agent.Result{Content: "Como posso te ajudar hoje?"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewRegistrationFailureWithoutToolGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}
