package workflows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type GoalEditDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestGoalEditDecisionsSuite(t *testing.T) {
	suite.Run(t, new(GoalEditDecisionsSuite))
}

func (s *GoalEditDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *GoalEditDecisionsSuite) baseState() GoalEditState {
	return GoalEditState{
		Status:       GoalEditActive,
		Awaiting:     GoalEditAwaitingConfirm,
		ResourceID:   "user-1",
		PreviousGoal: "Comprar uma casa",
		NewGoal:      "Viajar pela Europa",
		SuspendedAt:  s.now,
	}
}

func (s *GoalEditDecisionsSuite) TestDecideGoalEditConfirmation() {
	type args struct {
		state GoalEditState
		msg   PendingMessage
		now   time.Time
	}

	scenarios := []struct {
		name   string
		args   func() args
		expect GoalEditAction
	}{
		{
			name: "aceita sim",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: GoalEditActionAccept,
		},
		{
			name: "cancela nao",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "não", MessageID: "wamid-1"}, now: s.now}
			},
			expect: GoalEditActionCancel,
		},
		{
			name: "ambiguo primeira vez reprompta",
			args: func() args {
				state := s.baseState()
				state.RepromptCount = 0
				return args{state: state, msg: PendingMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: GoalEditActionReprompt,
		},
		{
			name: "ambiguo segunda vez cancela",
			args: func() args {
				state := s.baseState()
				state.RepromptCount = 1
				return args{state: state, msg: PendingMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: GoalEditActionCancel,
		},
		{
			name: "ttl expirado 15 minutos",
			args: func() args {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-16 * time.Minute)
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: GoalEditActionExpire,
		},
		{
			name: "replay de mensagem ja processada",
			args: func() args {
				state := s.baseState()
				state.MessageID = "wamid-1"
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: GoalEditActionReplay,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.args()
			action := DecideGoalEditConfirmation(a.state, a.msg, a.now)
			s.Equal(scenario.expect, action)
		})
	}
}

func (s *GoalEditDecisionsSuite) TestDecideGoalEditNewGoal() {
	scenarios := []struct {
		name       string
		text       string
		expectGoal string
		expectOK   bool
	}{
		{name: "texto valido", text: "  Viajar pela Europa  ", expectGoal: "Viajar pela Europa", expectOK: true},
		{name: "texto vazio invalido", text: "   ", expectGoal: "", expectOK: false},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			goal, ok := DecideGoalEditNewGoal(scenario.text)
			s.Equal(scenario.expectGoal, goal)
			s.Equal(scenario.expectOK, ok)
		})
	}
}
