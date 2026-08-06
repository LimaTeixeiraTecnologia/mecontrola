package workflows

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type CardManageEngineSuite struct {
	suite.Suite
	ctx       context.Context
	store     *wfStore
	engine    workflow.Engine[CardManageState]
	def       workflow.Definition[CardManageState]
	cardsMock *interfacemocks.CardManager
	idem      *fakeIdemWriter
	userID    uuid.UUID
}

func TestCardManageEngineSuite(t *testing.T) {
	suite.Run(t, new(CardManageEngineSuite))
}

func (s *CardManageEngineSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.cardsMock = interfacemocks.NewCardManager(s.T())
	s.idem = newFakeIdemWriter()
	s.engine = workflow.NewEngine[CardManageState](s.store, fake.NewProvider())
	s.def = BuildCardManageWorkflowWithObservability(s.cardsMock, s.idem, nil)
	s.userID = uuid.New()
}

func (s *CardManageEngineSuite) TestJornadaCartaoXPPeloEngineReal() {
	s.cardsMock.EXPECT().BankRecognized(mock.Anything, "XP").Return(false, nil).Once()
	s.cardsMock.EXPECT().
		CreateCard(mock.Anything, mock.MatchedBy(func(c interfaces.NewCard) bool {
			return c.Nickname == "Roxinho" && c.Bank == "XP" && c.DueDay == 10 &&
				c.ClosingDay == 10 && c.ClosingDayProvided
		})).
		Return(interfaces.CardRef{ID: uuid.New().String()}, nil).
		Once()

	key := CardManageKey(s.userID.String(), "+5511930111763")
	initial := CardManageState{
		Status:         CardManageActive,
		Operation:      CardManageOpCreate,
		UserID:         s.userID,
		DueDay:         10,
		DueDayProvided: true,
		MessageID:      "wamid-inicial",
	}

	result, err := s.engine.Start(s.ctx, s.def, key, initial)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(CardManageAwaitingNickname, result.State.Awaiting)

	turns := []struct {
		text     string
		msgID    string
		awaiting CardManageAwaiting
	}{
		{text: "Roxinho", msgID: "wamid-1", awaiting: CardManageAwaitingBank},
		{text: "O banco é XP mesmo.", msgID: "wamid-2", awaiting: CardManageAwaitingClosingDay},
		{text: "Dia 10", msgID: "wamid-3", awaiting: CardManageAwaitingClosingDay},
		{text: "10", msgID: "wamid-4", awaiting: CardManageAwaitingConfirm},
	}

	for _, turn := range turns {
		handled, reply, resumeErr := ContinueCardManage(s.ctx, s.engine, s.def, key, turn.text, turn.msgID)
		s.Require().NoError(resumeErr)
		s.True(handled, "turno %q deveria ser tratado pelo resume", turn.text)
		s.NotEmpty(reply)

		snap, ok, loadErr := s.store.Load(s.ctx, CardManageWorkflowID, key)
		s.Require().NoError(loadErr)
		s.Require().True(ok)
		s.Equal(workflow.RunStatusSuspended, snap.Status)

		state, _, found, stateErr := s.engine.LoadLatestState(s.ctx, s.def, key)
		s.Require().NoError(stateErr)
		s.Require().True(found)
		s.Equal(turn.awaiting, state.Awaiting, "awaiting deve sobreviver ao round-trip JSON apos %q", turn.text)
	}

	handled, reply, resumeErr := ContinueCardManage(s.ctx, s.engine, s.def, key, "Pode cadastrar sim.", "wamid-5")
	s.Require().NoError(resumeErr)
	s.True(handled)
	s.Contains(reply, "Roxinho")

	snap, ok, loadErr := s.store.Load(s.ctx, CardManageWorkflowID, key)
	s.Require().NoError(loadErr)
	s.Require().True(ok)
	s.Equal(workflow.RunStatusSucceeded, snap.Status)
}

func (s *CardManageEngineSuite) TestMensagemNaoRelacionadaDevolveHandledFalse() {
	key := CardManageKey(s.userID.String(), "+5511930111763")
	initial := CardManageState{
		Status:         CardManageActive,
		Operation:      CardManageOpCreate,
		UserID:         s.userID,
		DueDay:         10,
		DueDayProvided: true,
		MessageID:      "wamid-inicial",
	}

	result, err := s.engine.Start(s.ctx, s.def, key, initial)
	s.Require().NoError(err)
	s.Equal(CardManageAwaitingNickname, result.State.Awaiting)

	handled, reply, resumeErr := ContinueCardManage(s.ctx, s.engine, s.def, key, "gastei 30 no uber", "wamid-despesa")

	s.Require().NoError(resumeErr)
	s.False(handled, "despesa nova nao pode ser consumida como apelido do cartao")
	s.Empty(reply)

	snap, ok, loadErr := s.store.Load(s.ctx, CardManageWorkflowID, key)
	s.Require().NoError(loadErr)
	s.Require().True(ok)
	s.NotEqual(workflow.RunStatusSuspended, snap.Status)
}

func (s *CardManageEngineSuite) TestRunLiberadoNaoBloqueiaNovoCadastro() {
	key := CardManageKey(s.userID.String(), "+5511930111763")
	initial := CardManageState{
		Status:         CardManageActive,
		Operation:      CardManageOpCreate,
		UserID:         s.userID,
		DueDay:         10,
		DueDayProvided: true,
		MessageID:      "wamid-inicial",
	}

	_, err := s.engine.Start(s.ctx, s.def, key, initial)
	s.Require().NoError(err)

	handled, _, resumeErr := ContinueCardManage(s.ctx, s.engine, s.def, key, "gastei 30 no uber", "wamid-despesa")
	s.Require().NoError(resumeErr)
	s.False(handled)

	second := initial
	second.MessageID = "wamid-novo"
	result, startErr := s.engine.Start(s.ctx, s.def, key, second)

	s.Require().NoError(startErr, "run liberado nao pode bloquear um novo cadastro pelo indice unico")
	s.Equal(CardManageAwaitingNickname, result.State.Awaiting)
}
