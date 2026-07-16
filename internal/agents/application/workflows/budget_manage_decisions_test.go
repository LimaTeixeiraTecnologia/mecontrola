package workflows

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type BudgetManageDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestBudgetManageDecisionsSuite(t *testing.T) {
	suite.Run(t, new(BudgetManageDecisionsSuite))
}

func (s *BudgetManageDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *BudgetManageDecisionsSuite) baseState() BudgetManageState {
	return BudgetManageState{
		Status:      BudgetManageActive,
		Operation:   BudgetManageOpEditTotal,
		TotalCents:  350000,
		SuspendedAt: s.now,
	}
}

func (s *BudgetManageDecisionsSuite) TestDecideBudgetManagePostWrite() {
	scenarios := []struct {
		name       string
		resourceID string
		expectStep workflow.StepStatus
		expectErr  bool
	}{
		{
			name:       "id vazio sem recurso eh falso sucesso",
			resourceID: "",
			expectStep: workflow.StepStatusFailed,
			expectErr:  true,
		},
		{
			name:       "id em branco eh falso sucesso",
			resourceID: "   ",
			expectStep: workflow.StepStatusFailed,
			expectErr:  true,
		},
		{
			name:       "id valido completa",
			resourceID: "8f2b1c5e-0000-0000-0000-000000000001",
			expectStep: workflow.StepStatusCompleted,
			expectErr:  false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step, err := DecideBudgetManagePostWrite(scenario.resourceID)
			s.Equal(scenario.expectStep, step)
			if scenario.expectErr {
				s.Error(err)
				s.True(errors.Is(err, ErrBudgetManageAcceptedWithoutResource))
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *BudgetManageDecisionsSuite) TestDecideBudgetManageTotal() {
	scenarios := []struct {
		name       string
		totalCents int64
		expect     BudgetManageAction
	}{
		{name: "valor positivo preenche total", totalCents: 350000, expect: BudgetManageActionFillTotal},
		{name: "valor zero reprompta", totalCents: 0, expect: BudgetManageActionRepromptTotal},
		{name: "valor negativo reprompta", totalCents: -100, expect: BudgetManageActionRepromptTotal},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideBudgetManageTotal(scenario.totalCents)
			s.Equal(scenario.expect, decision.Action)
		})
	}
}

func (s *BudgetManageDecisionsSuite) TestDecideBudgetManageDistribution() {
	scenarios := []struct {
		name        string
		allocations map[string]int
		expect      BudgetManageAction
	}{
		{
			name:        "soma 10000 avanca para confirmacao",
			allocations: map[string]int{"a": 5000, "b": 5000},
			expect:      BudgetManageActionAdvanceToConfirm,
		},
		{
			name:        "soma diferente de 10000 reprompta",
			allocations: map[string]int{"a": 3000, "b": 3000},
			expect:      BudgetManageActionRepromptDistribution,
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideBudgetManageDistribution(scenario.allocations)
			s.Equal(scenario.expect, decision.Action)
		})
	}
}

func (s *BudgetManageDecisionsSuite) TestDecideBudgetManageConfirmation() {
	type args struct {
		state BudgetManageState
		msg   BudgetManageMessage
		now   time.Time
	}

	scenarios := []struct {
		name   string
		args   func() args
		expect BudgetManageAction
	}{
		{
			name: "confirma com sim",
			args: func() args {
				return args{state: s.baseState(), msg: BudgetManageMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: BudgetManageActionConfirm,
		},
		{
			name: "cancela com nao",
			args: func() args {
				return args{state: s.baseState(), msg: BudgetManageMessage{Text: "não", MessageID: "wamid-1"}, now: s.now}
			},
			expect: BudgetManageActionCancel,
		},
		{
			name: "ambiguo primeira vez reprompta",
			args: func() args {
				state := s.baseState()
				state.RepromptCount = 0
				return args{state: state, msg: BudgetManageMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: BudgetManageActionRepromptConfirm,
		},
		{
			name: "ambiguo segunda vez cancela",
			args: func() args {
				state := s.baseState()
				state.RepromptCount = 1
				return args{state: state, msg: BudgetManageMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: BudgetManageActionCancel,
		},
		{
			name: "ttl expirado",
			args: func() args {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-31 * time.Minute)
				return args{state: state, msg: BudgetManageMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: BudgetManageActionExpire,
		},
		{
			name: "replay de mensagem ja processada",
			args: func() args {
				state := s.baseState()
				state.MessageID = "wamid-1"
				return args{state: state, msg: BudgetManageMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: BudgetManageActionReplay,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.args()
			decision := DecideBudgetManageConfirmation(a.state, a.msg, a.now)
			s.Equal(scenario.expect, decision.Action)
		})
	}
}
