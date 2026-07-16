package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type CardManageDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestCardManageDecisionsSuite(t *testing.T) {
	suite.Run(t, new(CardManageDecisionsSuite))
}

func (s *CardManageDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *CardManageDecisionsSuite) baseState() CardManageState {
	return CardManageState{
		Status:      CardManageActive,
		Operation:   CardManageOpCreate,
		UserID:      uuid.New(),
		Nickname:    "Nubank",
		Bank:        "nubank",
		DueDay:      10,
		SuspendedAt: s.now,
	}
}

func (s *CardManageDecisionsSuite) TestDecideCardManageConfirmation() {
	type args struct {
		state CardManageState
		msg   PendingMessage
		now   time.Time
	}

	scenarios := []struct {
		name   string
		args   func() args
		expect CardManageAction
	}{
		{
			name: "aceita sim",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardManageActionAccept,
		},
		{
			name: "cancela nao",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "não", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardManageActionCancel,
		},
		{
			name: "ambiguo primeira vez reprompta",
			args: func() args {
				state := s.baseState()
				state.ConfirmReprompt = 0
				return args{state: state, msg: PendingMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardManageActionReprompt,
		},
		{
			name: "ambiguo segunda vez cancela",
			args: func() args {
				state := s.baseState()
				state.ConfirmReprompt = 1
				return args{state: state, msg: PendingMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardManageActionCancel,
		},
		{
			name: "ttl expirado 15 minutos",
			args: func() args {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-16 * time.Minute)
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardManageActionExpire,
		},
		{
			name: "replay de mensagem ja processada",
			args: func() args {
				state := s.baseState()
				state.ProcessedMessageID = "wamid-1"
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardManageActionReplay,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.args()
			action := DecideCardManageConfirmation(a.state, a.msg, a.now)
			s.Equal(scenario.expect, action)
		})
	}
}
