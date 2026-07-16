package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type DestructiveManageDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestDestructiveManageDecisionsSuite(t *testing.T) {
	suite.Run(t, new(DestructiveManageDecisionsSuite))
}

func (s *DestructiveManageDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *DestructiveManageDecisionsSuite) baseState() DestructiveManageState {
	return DestructiveManageState{
		Status:      DestructiveManageActive,
		Operation:   DestructiveOpDeleteCard,
		UserID:      uuid.New(),
		TargetRef:   uuid.New().String(),
		ImpactNote:  "Remoção permanente do 💳.",
		SuspendedAt: s.now,
	}
}

func (s *DestructiveManageDecisionsSuite) TestDecideDestructiveManageConfirmation() {
	type args struct {
		state DestructiveManageState
		msg   PendingMessage
		now   time.Time
	}

	scenarios := []struct {
		name   string
		args   func() args
		expect DestructiveManageAction
	}{
		{
			name: "aceita sim",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "sim"}, now: s.now}
			},
			expect: DestructiveManageActionAccept,
		},
		{
			name: "cancela nao",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "não"}, now: s.now}
			},
			expect: DestructiveManageActionCancel,
		},
		{
			name: "ambiguo primeira vez reprompta",
			args: func() args {
				state := s.baseState()
				state.RepromptDone = false
				return args{state: state, msg: PendingMessage{Text: "talvez"}, now: s.now}
			},
			expect: DestructiveManageActionReprompt,
		},
		{
			name: "ambiguo segunda vez cancela",
			args: func() args {
				state := s.baseState()
				state.RepromptDone = true
				return args{state: state, msg: PendingMessage{Text: "talvez"}, now: s.now}
			},
			expect: DestructiveManageActionCancel,
		},
		{
			name: "ttl expirado 5 minutos",
			args: func() args {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-6 * time.Minute)
				return args{state: state, msg: PendingMessage{Text: "sim"}, now: s.now}
			},
			expect: DestructiveManageActionExpire,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.args()
			action := DecideDestructiveManageConfirmation(a.state, a.msg, a.now)
			s.Equal(scenario.expect, action)
		})
	}
}
