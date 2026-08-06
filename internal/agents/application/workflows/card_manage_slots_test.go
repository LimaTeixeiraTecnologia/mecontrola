package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type CardManageSlotsSuite struct {
	suite.Suite
	ctx       context.Context
	cardsMock *interfacemocks.CardManager
	idem      *fakeIdemWriter
	userID    uuid.UUID
}

func TestCardManageSlotsSuite(t *testing.T) {
	suite.Run(t, new(CardManageSlotsSuite))
}

func (s *CardManageSlotsSuite) SetupTest() {
	s.ctx = context.Background()
	s.cardsMock = interfacemocks.NewCardManager(s.T())
	s.idem = newFakeIdemWriter()
	s.userID = uuid.New()
}

func (s *CardManageSlotsSuite) resume(def workflow.Definition[CardManageState], state CardManageState, text, messageID string) workflow.StepOutput[CardManageState] {
	state.ResumeText = text
	state.IncomingMessageID = messageID
	out, err := def.Root.Execute(s.ctx, state)
	s.Require().NoError(err)
	return out
}

func (s *CardManageSlotsSuite) TestProducaoCartaoXPCompletaSemLoop() {
	s.cardsMock.EXPECT().BankRecognized(mock.Anything, "XP").Return(false, nil).Once()
	s.cardsMock.EXPECT().
		CreateCard(mock.Anything, mock.MatchedBy(func(c interfaces.NewCard) bool {
			return c.Nickname == "Roxinho" && c.Bank == "XP" && c.DueDay == 10 && c.ClosingDay == 10 && c.ClosingDayProvided
		})).
		Return(interfaces.CardRef{ID: uuid.New().String()}, nil).
		Once()

	def := BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)

	initial := CardManageState{
		Status:         CardManageActive,
		Operation:      CardManageOpCreate,
		UserID:         s.userID,
		DueDay:         10,
		DueDayProvided: true,
		MessageID:      "wamid-inicial",
	}

	out, err := def.Root.Execute(s.ctx, initial)
	s.Require().NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(CardManageAwaitingNickname, out.State.Awaiting)

	out = s.resume(def, out.State, "Roxinho", "wamid-1")
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(CardManageAwaitingBank, out.State.Awaiting)
	s.Equal("Roxinho", out.State.Nickname)

	out = s.resume(def, out.State, "O banco é XP mesmo.", "wamid-2")
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(CardManageAwaitingClosingDay, out.State.Awaiting)
	s.Equal("XP", out.State.Bank)
	s.True(out.State.BankChecked)
	s.False(out.State.BankRecognized)

	out = s.resume(def, out.State, "Dia 10", "wamid-3")
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(CardManageAwaitingClosingDay, out.State.Awaiting)
	s.Equal(1, out.State.ClosingDayEcho)
	s.Contains(out.State.ResponseText, "fechamento")

	out = s.resume(def, out.State, "10", "wamid-4")
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(CardManageAwaitingConfirm, out.State.Awaiting)
	s.Equal(10, out.State.ClosingDay)
	s.True(out.State.ClosingDayProvided)

	out = s.resume(def, out.State, "Pode cadastrar sim.", "wamid-5")
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(CardManageCompleted, out.State.Status)
	s.Contains(out.State.ResponseText, "Roxinho")
}

func (s *CardManageSlotsSuite) TestBancoReconhecidoDescartaClosingDayDoLLM() {
	s.cardsMock.EXPECT().BankRecognized(mock.Anything, "Nubank").Return(true, nil).Once()

	def := BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)
	state := CardManageState{
		Status:             CardManageActive,
		Operation:          CardManageOpCreate,
		UserID:             s.userID,
		Nickname:           "Nu",
		NicknameProvided:   true,
		Bank:               "Nubank",
		BankProvided:       true,
		DueDay:             10,
		DueDayProvided:     true,
		ClosingDay:         5,
		ClosingDayProvided: true,
		MessageID:          "wamid-1",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.Require().NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(CardManageAwaitingConfirm, out.State.Awaiting)
	s.False(out.State.ClosingDayProvided)
	s.Equal(0, out.State.ClosingDay)
}

func (s *CardManageSlotsSuite) TestRunLegadoSemAwaitingVaiParaConfirm() {
	s.cardsMock.EXPECT().
		CreateCard(mock.Anything, mock.Anything).
		Return(interfaces.CardRef{ID: uuid.New().String()}, nil).
		Once()

	def := BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)
	legacy := CardManageState{
		Status:            CardManageActive,
		Operation:         CardManageOpCreate,
		UserID:            s.userID,
		Nickname:          "Nu",
		NicknameProvided:  true,
		Bank:              "Nubank",
		BankProvided:      true,
		DueDay:            10,
		DueDayProvided:    true,
		MessageID:         "wamid-legado",
		SuspendedAt:       time.Now().UTC(),
		ResumeText:        "sim",
		IncomingMessageID: "wamid-confirm",
	}

	out, err := def.Root.Execute(s.ctx, legacy)

	s.Require().NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(CardManageCompleted, out.State.Status)
}

func (s *CardManageSlotsSuite) TestSlotExpiradoNaoFicaSuspensoParaSempre() {
	def := BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)
	state := CardManageState{
		Status:            CardManageActive,
		Operation:         CardManageOpCreate,
		UserID:            s.userID,
		Awaiting:          CardManageAwaitingNickname,
		SuspendedAt:       time.Now().UTC().Add(-2 * cardManageSlotTTL),
		ResumeText:        "Roxinho",
		IncomingMessageID: "wamid-tarde",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.Require().NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(CardManageExpired, out.State.Status)
	s.True(out.State.Expired)
}

func (s *CardManageSlotsSuite) TestSlotCancelaAposRepromptsEsgotados() {
	def := BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)
	state := CardManageState{
		Status:      CardManageActive,
		Operation:   CardManageOpCreate,
		UserID:      s.userID,
		Awaiting:    CardManageAwaitingDueDay,
		SuspendedAt: time.Now().UTC(),
	}

	out := s.resume(def, state, "sei lá", "wamid-1")
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.SlotReprompt)

	out = s.resume(def, out.State, "hummm", "wamid-2")
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(CardManageCancelled, out.State.Status)
}

func (s *CardManageSlotsSuite) TestMensagemNaoRelacionadaLiberaOFluxoDeCartao() {
	def := BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)
	state := CardManageState{
		Status:      CardManageActive,
		Operation:   CardManageOpCreate,
		UserID:      s.userID,
		Awaiting:    CardManageAwaitingNickname,
		SuspendedAt: time.Now().UTC(),
	}

	out := s.resume(def, state, "gastei 30 no uber", "wamid-despesa")

	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.True(out.State.Released)
	s.Empty(out.State.ResponseText)
	s.NotEqual("gastei 30 no uber", out.State.Nickname)
}

func (s *CardManageSlotsSuite) TestShouldReleaseCardManageSlot() {
	scenarios := []struct {
		name   string
		text   string
		expect bool
	}{
		{name: "despesa nova libera", text: "gastei 30 no uber", expect: true},
		{name: "receita nova libera", text: "recebi 200 de freela", expect: true},
		{name: "consulta com verbo libera", text: "quanto gastei esse mes", expect: true},
		{name: "pergunta libera", text: "qual é a fatura do meu cartão?", expect: true},
		{name: "apelido normal nao libera", text: "Roxinho", expect: false},
		{name: "banco com lead-in nao libera", text: "O banco é XP mesmo.", expect: false},
		{name: "dia nao libera", text: "Dia 10", expect: false},
		{name: "confirmacao nao libera", text: "sim", expect: false},
		{name: "cancelamento nao libera", text: "cancela", expect: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, ShouldReleaseCardManageSlot(scenario.text))
		})
	}
}

func (s *CardManageSlotsSuite) TestNormalizeCardSlotAnswer() {
	scenarios := []struct {
		name   string
		text   string
		expect string
	}{
		{name: "producao: banco com lead-in e trailer acentuado", text: "O banco é XP mesmo.", expect: "XP"},
		{name: "apelido simples", text: "Roxinho", expect: "Roxinho"},
		{name: "apelido com lead-in", text: "O apelido é Roxinho", expect: "Roxinho"},
		{name: "vou chamar de", text: "vou chamar de Roxinho", expect: "Roxinho"},
		{name: "banco composto preserva acento", text: "o banco é Itaú Unibanco", expect: "Itaú Unibanco"},
		{name: "somente lead-in vira vazio", text: "o banco é", expect: ""},
		{name: "texto vazio", text: "   ", expect: ""},
		{name: "pontuacao isolada", text: "...", expect: ""},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, normalizeCardSlotAnswer(scenario.text))
		})
	}
}

func (s *CardManageSlotsSuite) TestDecideCardManageNextSlot() {
	base := CardManageState{Operation: CardManageOpCreate}

	scenarios := []struct {
		name   string
		state  CardManageState
		expect CardManageAwaiting
	}{
		{name: "sem apelido", state: base, expect: CardManageAwaitingNickname},
		{
			name:   "sem banco",
			state:  CardManageState{Nickname: "Nu"},
			expect: CardManageAwaitingBank,
		},
		{
			name:   "sem vencimento",
			state:  CardManageState{Nickname: "Nu", Bank: "XP"},
			expect: CardManageAwaitingDueDay,
		},
		{
			name:   "banco desconhecido sem fechamento",
			state:  CardManageState{Nickname: "Nu", Bank: "XP", DueDay: 10, BankChecked: true},
			expect: CardManageAwaitingClosingDay,
		},
		{
			name:   "banco reconhecido vai direto para confirmacao",
			state:  CardManageState{Nickname: "Nu", Bank: "Nubank", DueDay: 10, BankChecked: true, BankRecognized: true},
			expect: CardManageAwaitingConfirm,
		},
		{
			name:   "banco desconhecido com fechamento vai para confirmacao",
			state:  CardManageState{Nickname: "Nu", Bank: "XP", DueDay: 10, BankChecked: true, ClosingDay: 3, ClosingDayProvided: true},
			expect: CardManageAwaitingConfirm,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, DecideCardManageNextSlot(scenario.state))
		})
	}
}

func (s *CardManageSlotsSuite) TestDecideCardManageClosingDayAnswer() {
	state := CardManageState{DueDay: 10}

	scenarios := []struct {
		name   string
		state  CardManageState
		text   string
		expect CardManageSlotAction
		day    int
	}{
		{name: "dia igual ao vencimento desambigua uma vez", state: state, text: "Dia 10", expect: CardManageSlotDisambiguate, day: 10},
		{
			name:   "repeticao apos desambiguacao aceita",
			state:  CardManageState{DueDay: 10, ClosingDayEcho: 1},
			text:   "10",
			expect: CardManageSlotFill,
			day:    10,
		},
		{name: "dia diferente aceita direto", state: state, text: "dia 3", expect: CardManageSlotFill, day: 3},
		{name: "resposta sem numero repergunta", state: state, text: "não sei bem", expect: CardManageSlotReprompt},
		{name: "cancelamento explicito", state: state, text: "cancela", expect: CardManageSlotCancel},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideCardManageClosingDayAnswer(scenario.state, scenario.text)
			s.Equal(scenario.expect, decision.Action)
			if scenario.day > 0 {
				s.Equal(scenario.day, decision.Day)
			}
		})
	}
}

func (s *CardManageSlotsSuite) TestCardManageAwaitingIsClosedType() {
	scenarios := []struct {
		name     string
		awaiting CardManageAwaiting
		label    string
		valid    bool
	}{
		{name: "nickname", awaiting: CardManageAwaitingNickname, label: "nickname", valid: true},
		{name: "bank", awaiting: CardManageAwaitingBank, label: "bank", valid: true},
		{name: "due_day", awaiting: CardManageAwaitingDueDay, label: "due_day", valid: true},
		{name: "closing_day", awaiting: CardManageAwaitingClosingDay, label: "closing_day", valid: true},
		{name: "confirm", awaiting: CardManageAwaitingConfirm, label: "confirm", valid: true},
		{name: "zero value invalido", awaiting: CardManageAwaiting(0), label: "unknown", valid: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.label, scenario.awaiting.String())
			s.Equal(scenario.valid, scenario.awaiting.IsValid())
			if scenario.valid {
				parsed, err := ParseCardManageAwaiting(scenario.label)
				s.NoError(err)
				s.Equal(scenario.awaiting, parsed)
			}
		})
	}
}
