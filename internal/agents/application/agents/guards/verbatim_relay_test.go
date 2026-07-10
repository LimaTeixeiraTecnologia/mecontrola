package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type VerbatimRelayGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestVerbatimRelayGuardSuite(t *testing.T) {
	suite.Run(t, new(VerbatimRelayGuardSuite))
}

func (s *VerbatimRelayGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *VerbatimRelayGuardSuite) TestName() {
	guard := NewVerbatimRelayGuard()
	s.Equal("verbatim_relay", guard.Name())
}

func (s *VerbatimRelayGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "content diverge do message da tool verbatim -> forca override",
			args: args{out: agent.Result{
				Content: "resposta parafraseada pelo modelo",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"clarify","message":"Qual categoria você quer usar?"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal("Qual categoria você quer usar?", decision.Result.Content)
			},
		},
		{
			name: "content ja igual ao verbatim esperado -> nao trata",
			args: args{out: agent.Result{
				Content: "Qual categoria você quer usar?",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"clarify","message":"Qual categoria você quer usar?"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "impactNote de edit_entry diverge -> forca override",
			args: args{out: agent.Result{
				Content: "Vou editar seu lançamento",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "edit_entry", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"needsConfirmation":true,"impactNote":"Confirma a edição deste lançamento?","outcome":"clarify"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal("Confirma a edição deste lançamento?", decision.Result.Content)
			},
		},
		{
			name: "tool sem campo verbatim -> nao trata",
			args: args{out: agent.Result{
				Content: "Aqui está seu resumo do mês",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "query_month", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"entries":[]}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "sem tool calls -> nao trata",
			args: args{out: agent.Result{Content: "resposta livre"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "tool call com erro nao fornece verbatim -> nao trata",
			args: args{out: agent.Result{
				Content: "Não consegui registrar. Tente novamente em breve.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeError, Content: `tool register_expense: falha interna`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewVerbatimRelayGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}
