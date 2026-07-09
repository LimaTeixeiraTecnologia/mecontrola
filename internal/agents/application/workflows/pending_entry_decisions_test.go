package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func baseState() PendingEntryState {
	return PendingEntryState{
		Status:      PendingStatusActive,
		Awaiting:    AwaitingSlotCategory,
		SuspendedAt: time.Now().UTC(),
	}
}

type PendingEntryDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestPendingEntryDecisionsSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryDecisionsSuite))
}

func (s *PendingEntryDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *PendingEntryDecisionsSuite) baseState() PendingEntryState {
	state := baseState()
	state.SuspendedAt = s.now
	return state
}

func (s *PendingEntryDecisionsSuite) resumeState() PendingEntryState {
	state := s.baseState()
	state.MessageID = "wamid-001"
	return state
}

func (s *PendingEntryDecisionsSuite) decisionMsg(text string) PendingMessage {
	return PendingMessage{Text: text, MessageID: "wamid-new"}
}

func makeCandidate(rootSlug, subSlug string) PendingCategoryCandidate {
	return PendingCategoryCandidate{
		RootCategoryID:  uuid.New(),
		RootSlug:        rootSlug,
		SubcategoryID:   uuid.New(),
		SubcategorySlug: subSlug,
		Path:            rootSlug + " > " + subSlug,
	}
}

func (s *PendingEntryDecisionsSuite) TestDecideNewOperationReplacement() {
	type args struct {
		text string
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision PendingDecision)
	}{
		{
			name: "G7-01 substitui por frase completa",
			args: args{text: "Gastei R$ 150,00 na farmácia hoje, no pix"},
			expect: func(decision PendingDecision) {
				s.Equal(PendingActionReplace, decision.Action)
			},
		},
		{
			name: "termo simples nao substitui",
			args: args{text: "supermercado"},
			expect: func(decision PendingDecision) {
				s.Equal(PendingActionNone, decision.Action)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideNewOperationReplacement(s.baseState(), s.decisionMsg(scenario.args.text))
			scenario.expect(decision)
		})
	}
}

func (s *PendingEntryDecisionsSuite) TestDecidePendingResume() {
	type args struct {
		mutate func(state *PendingEntryState)
		text   string
		now    time.Time
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision PendingDecision, err error)
	}{
		{
			name: "G7-08 expirado",
			args: args{
				mutate: func(state *PendingEntryState) { state.SuspendedAt = s.now.Add(-31 * time.Minute) },
				text:   "supermercado",
				now:    s.now,
			},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionExpire, decision.Action)
			},
		},
		{
			name: "G7-04 cancela",
			args: args{text: "cancela", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionCancel, decision.Action)
			},
		},
		{
			name: "cancelar",
			args: args{text: "cancelar", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionCancel, decision.Action)
			},
		},
		{
			name: "G7-05 deixa pra la",
			args: args{text: "deixa pra lá", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionCancel, decision.Action)
			},
		},
		{
			name: "G7-06 nao registra",
			args: args{text: "não registra", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionCancel, decision.Action)
			},
		},
		{
			name: "replace nova operacao pix",
			args: args{text: "Gastei R$ 150,00 na farmácia hoje, no pix", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReplace, decision.Action)
			},
		},
		{
			name: "replace nova operacao mercado pix",
			args: args{text: "Gastei R$ 150,00 no mercado, pix", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReplace, decision.Action)
			},
		},
		{
			name: "replace nova operacao cartao",
			args: args{text: "Paguei R$ 50,00 no restaurante, cartão", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReplace, decision.Action)
			},
		},
		{
			name: "replace nova operacao recebi",
			args: args{text: "Recebi R$ 3000,00 de salário", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReplace, decision.Action)
			},
		},
		{
			name: "G7-13 reprompt primeiro",
			args: args{
				mutate: func(state *PendingEntryState) { state.RepromptCount = 0 },
				text:   "tudo bem",
				now:    s.now,
			},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReprompt, decision.Action)
			},
		},
		{
			name: "reprompt primeiro xpto",
			args: args{
				mutate: func(state *PendingEntryState) { state.RepromptCount = 0 },
				text:   "xpto",
				now:    s.now,
			},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReprompt, decision.Action)
			},
		},
		{
			name: "G7-14 cancela apos 2 reprompts",
			args: args{
				mutate: func(state *PendingEntryState) { state.RepromptCount = 1 },
				text:   "ok sim",
				now:    s.now,
			},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionCancel, decision.Action)
			},
		},
		{
			name: "reprompt max atingido cancela",
			args: args{
				mutate: func(state *PendingEntryState) { state.RepromptCount = 1 },
				text:   "xpto",
				now:    s.now,
			},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionCancel, decision.Action)
			},
		},
		{
			name: "G7-07 sim e pix reprompt",
			args: args{
				mutate: func(state *PendingEntryState) {
					state.Awaiting = AwaitingSlotCategory
					state.RepromptCount = 0
				},
				text: "sim e pix",
				now:  s.now,
			},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
				s.Equal(PendingActionReprompt, decision.Action)
			},
		},
		{
			name: "no IO",
			args: args{text: "supermercado", now: s.now},
			expect: func(decision PendingDecision, err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := s.resumeState()
			if scenario.args.mutate != nil {
				scenario.args.mutate(&state)
			}
			decision, err := DecidePendingResume(state, s.decisionMsg(scenario.args.text), scenario.args.now)
			scenario.expect(decision, err)
		})
	}
}

func (s *PendingEntryDecisionsSuite) TestDecideConfirmation() {
	type args struct {
		mutate func(state *PendingEntryState)
		msg    PendingMessage
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision ConfirmDecision, err error)
	}{
		{
			name: "accept sim",
			args: args{msg: PendingMessage{Text: "sim"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionAccept, decision.Action)
			},
		},
		{
			name: "accept confirmar",
			args: args{msg: PendingMessage{Text: "confirmar"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionAccept, decision.Action)
			},
		},
		{
			name: "accept confirma",
			args: args{msg: PendingMessage{Text: "confirma"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionAccept, decision.Action)
			},
		},
		{
			name: "accept ok",
			args: args{msg: PendingMessage{Text: "ok"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionAccept, decision.Action)
			},
		},
		{
			name: "accept pode",
			args: args{msg: PendingMessage{Text: "pode"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionAccept, decision.Action)
			},
		},
		{
			name: "cancel nao acento",
			args: args{msg: PendingMessage{Text: "não"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionCancel, decision.Action)
			},
		},
		{
			name: "cancel nao",
			args: args{msg: PendingMessage{Text: "nao"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionCancel, decision.Action)
			},
		},
		{
			name: "cancel cancela",
			args: args{msg: PendingMessage{Text: "cancela"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionCancel, decision.Action)
			},
		},
		{
			name: "cancel cancelar",
			args: args{msg: PendingMessage{Text: "cancelar"}},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionCancel, decision.Action)
			},
		},
		{
			name: "ambiguo reprompt",
			args: args{
				mutate: func(state *PendingEntryState) { state.ConfirmRepromptCount = 0 },
				msg:    PendingMessage{Text: "talvez"},
			},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionReprompt, decision.Action)
			},
		},
		{
			name: "ambiguo segunda vez cancela",
			args: args{
				mutate: func(state *PendingEntryState) { state.ConfirmRepromptCount = 1 },
				msg:    PendingMessage{Text: "sei lá"},
			},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionCancel, decision.Action)
			},
		},
		{
			name: "expirado",
			args: args{
				mutate: func(state *PendingEntryState) { state.SuspendedAt = s.now.Add(-31 * time.Minute) },
				msg:    PendingMessage{Text: "sim"},
			},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionExpire, decision.Action)
			},
		},
		{
			name: "replay",
			args: args{
				mutate: func(state *PendingEntryState) { state.ProcessedMessageID = "wamid-123" },
				msg:    PendingMessage{Text: "sim", MessageID: "wamid-123"},
			},
			expect: func(decision ConfirmDecision, err error) {
				s.NoError(err)
				s.Equal(ConfirmActionReplay, decision.Action)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := s.baseState()
			state.Awaiting = AwaitingSlotConfirmation
			if scenario.args.mutate != nil {
				scenario.args.mutate(&state)
			}
			decision, err := DecideConfirmation(state, scenario.args.msg, s.now)
			scenario.expect(decision, err)
		})
	}
}

func (s *PendingEntryDecisionsSuite) TestDecideCategoryChoice() {
	rootID := uuid.New()
	type args struct {
		candidates []PendingCategoryCandidate
		choice     string
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(decision CategoryChoiceDecision, err error)
	}{
		{
			name: "CA-15 por indice",
			args: args{
				candidates: []PendingCategoryCandidate{
					makeCandidate("custo-fixo", "plano-de-saude"),
					makeCandidate("custo-fixo", "consultas-e-exames"),
					makeCandidate("custo-fixo", "terapia-e-saude-mental"),
				},
				choice: "2",
			},
			expect: func(decision CategoryChoiceDecision, err error) {
				s.NoError(err)
				s.Equal(CategoryChoiceActionSelected, decision.Action)
				s.Equal("consultas-e-exames", decision.Candidate.SubcategorySlug)
			},
		},
		{
			name: "CA-15 por nome",
			args: args{
				candidates: []PendingCategoryCandidate{
					makeCandidate("custo-fixo", "plano-de-saude"),
					makeCandidate("custo-fixo", "consultas-e-exames"),
					makeCandidate("custo-fixo", "terapia-e-saude-mental"),
				},
				choice: "consultas-e-exames",
			},
			expect: func(decision CategoryChoiceDecision, err error) {
				s.NoError(err)
				s.Equal(CategoryChoiceActionSelected, decision.Action)
				s.Equal("consultas-e-exames", decision.Candidate.SubcategorySlug)
			},
		},
		{
			name: "G7-03 root only",
			args: args{
				candidates: []PendingCategoryCandidate{
					{
						RootCategoryID:  rootID,
						RootSlug:        "custo-fixo",
						SubcategoryID:   rootID,
						SubcategorySlug: "custo-fixo",
						Path:            "custo-fixo",
					},
				},
				choice: "1",
			},
			expect: func(decision CategoryChoiceDecision, err error) {
				s.NoError(err)
				s.Equal(CategoryChoiceActionRootOnly, decision.Action)
			},
		},
		{
			name: "reprompt incompativel",
			args: args{
				candidates: []PendingCategoryCandidate{
					makeCandidate("custo-fixo", "supermercado"),
				},
				choice: "textocompletoaleatório",
			},
			expect: func(decision CategoryChoiceDecision, err error) {
				s.NoError(err)
				s.Equal(CategoryChoiceActionReprompt, decision.Action)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision, err := DecideCategoryChoice(s.baseState(), scenario.args.candidates, scenario.args.choice)
			scenario.expect(decision, err)
		})
	}
}

func (s *PendingEntryDecisionsSuite) TestParseWeekday() {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	scenarios := []struct {
		text string
		want string
		ok   bool
	}{
		{"segunda", "2026-07-06", true},
		{"segunda-feira", "2026-07-06", true},
		{"terca", "2026-06-30", true},
		{"terca-feira", "2026-06-30", true},
		{"terça", "2026-06-30", true},
		{"terça-feira", "2026-06-30", true},
		{"quarta", "2026-07-01", true},
		{"quarta-feira", "2026-07-01", true},
		{"quinta", "2026-07-02", true},
		{"quinta-feira", "2026-07-02", true},
		{"sexta", "2026-07-03", true},
		{"sexta-feira", "2026-07-03", true},
		{"sabado", "2026-07-04", true},
		{"sábado", "2026-07-04", true},
		{"domingo", "2026-07-05", true},
		{"segunda passada", "2026-06-29", true},
		{"segunda-feira passada", "2026-06-29", true},
		{"sabado passado", "2026-06-27", true},
		{"sábado passado", "2026-06-27", true},
		{"domingo passado", "2026-06-28", true},
		{"semana passada", "", false},
		{"mes passado", "", false},
		{"mês passado", "", false},
		{"amanha", "", false},
		{"", "", false},
		{"hoje", "", false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.text, func() {
			got, ok := parseWeekday(scenario.text, now)
			s.Equal(scenario.ok, ok, "text=%q ok mismatch", scenario.text)
			s.Equal(scenario.want, got, "text=%q date mismatch", scenario.text)
		})
	}
}

func (s *PendingEntryDecisionsSuite) TestParseInputDate() {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	scenarios := []struct {
		text string
		want string
	}{
		{"segunda", "2026-07-06"},
		{"terça-feira", "2026-06-30"},
		{"sábado passado", "2026-06-27"},
		{"semana passada", ""},
		{"mês passado", ""},
		{"hoje", "2026-07-06"},
		{"ontem", "2026-07-05"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.text, func() {
			got := parseInputDate(scenario.text, now)
			s.Equal(scenario.want, got, "text=%q", scenario.text)
		})
	}
}

func (s *PendingEntryDecisionsSuite) TestKnownPaymentMethodsAllValuesParseValid() {
	for key, val := range knownPaymentMethods {
		_, err := valueobjects.ParsePaymentMethod(val)
		s.NoError(err, "key=%q value=%q deve ser aceito por ParsePaymentMethod", key, val)
	}
}

func (s *PendingEntryDecisionsSuite) TestKnownPaymentMethodsAllValuesParseValidForCreate() {
	for key, val := range knownPaymentMethods {
		_, err := valueobjects.ParsePaymentMethodForCreate(val)
		s.NoError(err, "key=%q value=%q deve ser aceito por ParsePaymentMethodForCreate (register_expense)", key, val)
	}
}

func (s *PendingEntryDecisionsSuite) TestRecognizePaymentMethod_Doc_MapsToTED() {
	s.Equal("ted", recognizePaymentMethod("doc"))
}
