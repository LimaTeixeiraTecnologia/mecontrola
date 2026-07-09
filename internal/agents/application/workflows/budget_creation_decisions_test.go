package workflows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type BudgetCreationDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestBudgetCreationDecisionsSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationDecisionsSuite))
}

func (s *BudgetCreationDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *BudgetCreationDecisionsSuite) baseState() BudgetCreationState {
	return BudgetCreationState{
		Status:      BudgetCreationActive,
		Awaiting:    AwaitingBudgetTotal,
		SuspendedAt: s.now,
	}
}

func (s *BudgetCreationDecisionsSuite) TestDecideBudgetTotal() {
	type args struct {
		totalCents int64
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision BudgetCreationDecision)
	}{
		{
			name: "total invalido zero reprompt",
			args: args{totalCents: 0},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionRepromptTotal, decision.Action)
			},
		},
		{
			name: "total invalido negativo reprompt",
			args: args{totalCents: -100},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionRepromptTotal, decision.Action)
			},
		},
		{
			name: "total valido preenche slot",
			args: args{totalCents: 500000},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionFillTotal, decision.Action)
				s.Equal(int64(500000), decision.TotalCents)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideBudgetTotal(scenario.args.totalCents)
			scenario.expect(decision)
		})
	}
}

func (s *BudgetCreationDecisionsSuite) TestDecideBudgetDistribution() {
	type args struct {
		allocations map[string]int
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision BudgetCreationDecision)
	}{
		{
			name: "distribuicao incompleta soma menor bloqueia",
			args: args{allocations: map[string]int{
				"expense.custo_fixo": 4000,
				"expense.metas":      1000,
			}},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionRepromptDistribution, decision.Action)
			},
		},
		{
			name: "distribuicao soma maior bloqueia",
			args: args{allocations: map[string]int{
				"expense.custo_fixo": 6000,
				"expense.metas":      6000,
			}},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionRepromptDistribution, decision.Action)
			},
		},
		{
			name: "distribuicao soma 10000 transita para confirmacao",
			args: args{allocations: map[string]int{
				"expense.custo_fixo":           4000,
				"expense.conhecimento":         1000,
				"expense.prazeres":             1000,
				"expense.metas":                1000,
				"expense.liberdade_financeira": 3000,
			}},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionAdvanceToConfirm, decision.Action)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideBudgetDistribution(scenario.args.allocations)
			scenario.expect(decision)
		})
	}
}

func (s *BudgetCreationDecisionsSuite) TestDecideBudgetPendingResume() {
	type args struct {
		mutate func(state *BudgetCreationState)
		msg    BudgetCreationMessage
		now    time.Time
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision BudgetCreationDecision, err error)
	}{
		{
			name: "expirado apos TTL de 30 minutos",
			args: args{
				mutate: func(state *BudgetCreationState) { state.SuspendedAt = s.now.Add(-31 * time.Minute) },
				msg:    BudgetCreationMessage{Text: "1000", MessageID: "wamid-1"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision, err error) {
				s.NoError(err)
				s.Equal(BudgetActionExpire, decision.Action)
			},
		},
		{
			name: "dentro do TTL nao expira",
			args: args{
				mutate: func(state *BudgetCreationState) { state.SuspendedAt = s.now.Add(-29 * time.Minute) },
				msg:    BudgetCreationMessage{Text: "1000", MessageID: "wamid-1"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision, err error) {
				s.NoError(err)
				s.NotEqual(BudgetActionExpire, decision.Action)
			},
		},
		{
			name: "replay de messageID ja processado",
			args: args{
				mutate: func(state *BudgetCreationState) { state.MessageID = "wamid-processed" },
				msg:    BudgetCreationMessage{Text: "1000", MessageID: "wamid-processed"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision, err error) {
				s.NoError(err)
				s.Equal(BudgetActionReplay, decision.Action)
			},
		},
		{
			name: "reprompt count no limite cancela",
			args: args{
				mutate: func(state *BudgetCreationState) { state.RepromptCount = budgetCreationMaxReprompts },
				msg:    BudgetCreationMessage{Text: "xpto", MessageID: "wamid-2"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision, err error) {
				s.NoError(err)
				s.Equal(BudgetActionCancel, decision.Action)
			},
		},
		{
			name: "sem condicao especial nao decide acao",
			args: args{
				msg: BudgetCreationMessage{Text: "1000", MessageID: "wamid-3"},
				now: s.now,
			},
			expect: func(decision BudgetCreationDecision, err error) {
				s.NoError(err)
				s.Equal(BudgetActionNone, decision.Action)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := s.baseState()
			if scenario.args.mutate != nil {
				scenario.args.mutate(&state)
			}
			decision, err := DecideBudgetPendingResume(state, scenario.args.msg, scenario.args.now)
			scenario.expect(decision, err)
		})
	}
}

func (s *BudgetCreationDecisionsSuite) TestDecideBudgetConfirmation() {
	type args struct {
		mutate func(state *BudgetCreationState)
		msg    BudgetCreationMessage
		now    time.Time
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision BudgetCreationDecision)
	}{
		{
			name: "expirado apos TTL encerra",
			args: args{
				mutate: func(state *BudgetCreationState) { state.SuspendedAt = s.now.Add(-31 * time.Minute) },
				msg:    BudgetCreationMessage{Text: "sim", MessageID: "wamid-1"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionExpire, decision.Action)
			},
		},
		{
			name: "replay de messageID ja processado nao efetiva de novo",
			args: args{
				mutate: func(state *BudgetCreationState) { state.MessageID = "wamid-done" },
				msg:    BudgetCreationMessage{Text: "sim", MessageID: "wamid-done"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionReplay, decision.Action)
			},
		},
		{
			name: "confirmacao explicita sim efetiva",
			args: args{
				msg: BudgetCreationMessage{Text: "sim", MessageID: "wamid-2"},
				now: s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionConfirm, decision.Action)
			},
		},
		{
			name: "confirmacao explicita confirmar efetiva",
			args: args{
				msg: BudgetCreationMessage{Text: "confirmar", MessageID: "wamid-2b"},
				now: s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionConfirm, decision.Action)
			},
		},
		{
			name: "cancelamento explicito nao encerra sem efeito",
			args: args{
				msg: BudgetCreationMessage{Text: "não", MessageID: "wamid-3"},
				now: s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionCancel, decision.Action)
			},
		},
		{
			name: "cancelamento explicito cancelar",
			args: args{
				msg: BudgetCreationMessage{Text: "cancela", MessageID: "wamid-3b"},
				now: s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionCancel, decision.Action)
			},
		},
		{
			name: "resposta ambigua primeira vez reprompt unico",
			args: args{
				mutate: func(state *BudgetCreationState) { state.RepromptCount = 0 },
				msg:    BudgetCreationMessage{Text: "talvez", MessageID: "wamid-4"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionRepromptConfirm, decision.Action)
			},
		},
		{
			name: "resposta ambigua segunda vez cancela",
			args: args{
				mutate: func(state *BudgetCreationState) { state.RepromptCount = budgetCreationMaxReprompts },
				msg:    BudgetCreationMessage{Text: "talvez", MessageID: "wamid-5"},
				now:    s.now,
			},
			expect: func(decision BudgetCreationDecision) {
				s.Equal(BudgetActionCancel, decision.Action)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := s.baseState()
			state.Awaiting = AwaitingBudgetConfirm
			if scenario.args.mutate != nil {
				scenario.args.mutate(&state)
			}
			decision := DecideBudgetConfirmation(state, scenario.args.msg, scenario.args.now)
			scenario.expect(decision)
		})
	}
}

func (s *BudgetCreationDecisionsSuite) TestCleanupDeterministicoAposConfirmCancelExpire() {
	terminalActions := map[BudgetCreationAction]bool{
		BudgetActionConfirm: true,
		BudgetActionCancel:  true,
		BudgetActionExpire:  true,
	}

	scenarios := []struct {
		name   string
		msg    BudgetCreationMessage
		mutate func(state *BudgetCreationState)
	}{
		{
			name: "confirm encerra",
			msg:  BudgetCreationMessage{Text: "sim", MessageID: "wamid-a"},
		},
		{
			name: "cancel encerra",
			msg:  BudgetCreationMessage{Text: "não", MessageID: "wamid-b"},
		},
		{
			name:   "expire encerra",
			msg:    BudgetCreationMessage{Text: "sim", MessageID: "wamid-c"},
			mutate: func(state *BudgetCreationState) { state.SuspendedAt = s.now.Add(-31 * time.Minute) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := s.baseState()
			state.Awaiting = AwaitingBudgetConfirm
			if scenario.mutate != nil {
				scenario.mutate(&state)
			}
			decision := DecideBudgetConfirmation(state, scenario.msg, s.now)
			s.True(terminalActions[decision.Action], "ação %v deveria ser terminal (confirm/cancel/expire)", decision.Action)
		})
	}
}
