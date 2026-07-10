package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type CardProvenanceGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCardProvenanceGuardSuite(t *testing.T) {
	suite.Run(t, new(CardProvenanceGuardSuite))
}

func (s *CardProvenanceGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CardProvenanceGuardSuite) TestName() {
	guard := NewCardProvenanceGuard()
	s.Equal("card_provenance", guard.Name())
}

func (s *CardProvenanceGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "register_expense sem resolve_card/list_cards prévio -> pede escolha",
			args: args{out: agent.Result{
				Content: "Lançamento registrado.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"routed"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(cardProvenanceFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeClarify, decision.Result.ToolOutcome)
			},
		},
		{
			name: "create_recurrence sem resolve_card/list_cards prévio -> pede escolha",
			args: args{out: agent.Result{
				Content: "Recorrência solicitada.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "create_recurrence", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"clarify"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
			},
		},
		{
			name: "query_card_invoice sem resolve_card/list_cards prévio -> pede escolha",
			args: args{out: agent.Result{
				Content: "Fatura consultada.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "query_card_invoice", Outcome: agent.ToolCallOutcomeSuccess, Content: `{}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
			},
		},
		{
			name: "resolve_card antes de register_expense -> não trata (caminho feliz)",
			args: args{out: agent.Result{
				Content: "Lançamento registrado.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "resolve_card", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"found":true,"cardId":"card-1"}`},
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"routed"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "list_cards antes de query_card_invoice -> não trata (caminho feliz)",
			args: args{out: agent.Result{
				Content: "Fatura consultada.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "list_cards", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"cards":[]}`},
					{Tool: "query_card_invoice", Outcome: agent.ToolCallOutcomeSuccess, Content: `{}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "resolve_card com found=false seguido de register_expense -> sequência satisfeita (payload é responsabilidade da validação de existência na tool)",
			args: args{out: agent.Result{
				Content: "Não encontrei esse cartão.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "resolve_card", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"found":false}`},
					{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"clarify"}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "register_expense depois de resolve_card em outra ordem sem consumidor -> não trata",
			args: args{out: agent.Result{
				Content: "Ok, aqui estão seus cartões.",
				ToolCalls: []agent.ToolCallRecord{
					{Tool: "resolve_card", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"found":true}`},
				},
			}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "sem tool calls -> não trata",
			args: args{out: agent.Result{Content: "Como posso ajudar?"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewCardProvenanceGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}
