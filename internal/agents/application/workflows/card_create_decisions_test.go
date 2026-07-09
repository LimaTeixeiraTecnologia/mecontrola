package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type CardCreateDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestCardCreateDecisionsSuite(t *testing.T) {
	suite.Run(t, new(CardCreateDecisionsSuite))
}

func (s *CardCreateDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *CardCreateDecisionsSuite) baseState() CardCreateState {
	return CardCreateState{
		Status:      CardCreateStatusActive,
		Awaiting:    AwaitingConfirm,
		UserID:      uuid.New(),
		Nickname:    "Nubank",
		Bank:        "nubank",
		DueDay:      10,
		SuspendedAt: s.now,
	}
}

func (s *CardCreateDecisionsSuite) TestDecideCardCreateConfirmation() {
	type args struct {
		state CardCreateState
		msg   PendingMessage
		now   time.Time
	}

	scenarios := []struct {
		name   string
		args   func() args
		expect CardConfirmAction
	}{
		{
			name: "aceita confirmacao explicita sim",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmAccept,
		},
		{
			name: "aceita confirma",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "confirmar", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmAccept,
		},
		{
			name: "aceita ok",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "ok", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmAccept,
		},
		{
			name: "aceita pode",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "pode", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmAccept,
		},
		{
			name: "cancela negacao explicita nao",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "não", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmCancel,
		},
		{
			name: "cancela cancelar",
			args: func() args {
				return args{state: s.baseState(), msg: PendingMessage{Text: "cancelar", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmCancel,
		},
		{
			name: "ambiguo primeira vez reprompt",
			args: func() args {
				state := s.baseState()
				state.ConfirmReprompt = 0
				return args{state: state, msg: PendingMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmReprompt,
		},
		{
			name: "ambiguo segunda vez cancela",
			args: func() args {
				state := s.baseState()
				state.ConfirmReprompt = 1
				return args{state: state, msg: PendingMessage{Text: "talvez", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmCancel,
		},
		{
			name: "ttl expirado 15 minutos",
			args: func() args {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-16 * time.Minute)
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmExpire,
		},
		{
			name: "dentro do ttl nao expira",
			args: func() args {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-14 * time.Minute)
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmAccept,
		},
		{
			name: "replay de mensagem ja processada",
			args: func() args {
				state := s.baseState()
				state.ProcessedMessageID = "wamid-1"
				return args{state: state, msg: PendingMessage{Text: "sim", MessageID: "wamid-1"}, now: s.now}
			},
			expect: CardConfirmReplay,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.args()
			action := DecideCardCreateConfirmation(a.state, a.msg, a.now)
			s.Equal(scenario.expect, action)
		})
	}
}

func (s *CardCreateDecisionsSuite) TestCardConfirmActionRoundTrip() {
	type args struct {
		action CardConfirmAction
		text   string
	}

	scenarios := []struct {
		name string
		args args
	}{
		{name: "accept", args: args{action: CardConfirmAccept, text: "accept"}},
		{name: "cancel", args: args{action: CardConfirmCancel, text: "cancel"}},
		{name: "reprompt", args: args{action: CardConfirmReprompt, text: "reprompt"}},
		{name: "expire", args: args{action: CardConfirmExpire, text: "expire"}},
		{name: "replay", args: args{action: CardConfirmReplay, text: "replay"}},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.args.text, scenario.args.action.String())
			s.True(scenario.args.action.IsValid())
		})
	}
}

func (s *CardCreateDecisionsSuite) TestCardConfirmActionIsValidRejectsOutOfRange() {
	s.False(CardConfirmAction(0).IsValid())
	s.False(CardConfirmAction(99).IsValid())
}

func (s *CardCreateDecisionsSuite) TestCardConfirmActionStringUnknown() {
	s.Equal("unknown", CardConfirmAction(0).String())
	s.Equal("unknown", CardConfirmAction(99).String())
}
