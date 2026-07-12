package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

var errCardCreateInfraStub = errors.New("workflows: falha de infraestrutura simulada")

type cardCreateIdempotentResult struct {
	id      uuid.UUID
	outcome agent.ToolOutcome
	err     error
	invoke  bool
}

type fakeCardCreateIdempotentWriter struct {
	calls   int
	results []cardCreateIdempotentResult
}

func (f *fakeCardCreateIdempotentWriter) Execute(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ int,
	_ string,
	_ string,
	write IdempotentWriteFn,
	_ DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	r := f.results[f.calls]
	f.calls++
	if r.invoke {
		id, _, writeErr := write(context.Background())
		if writeErr != nil {
			return uuid.Nil, 0, writeErr
		}
		return id, r.outcome, nil
	}
	return r.id, r.outcome, r.err
}

type CardCreateConfirmWorkflowSuite struct {
	suite.Suite
	ctx    context.Context
	store  *wfStore
	engine workflow.Engine[CardCreateState]
	cards  *imocks.CardManager
	idem   *fakeCardCreateIdempotentWriter
	def    workflow.Definition[CardCreateState]
	userID uuid.UUID
	key    string
}

func TestCardCreateConfirmWorkflowSuite(t *testing.T) {
	suite.Run(t, new(CardCreateConfirmWorkflowSuite))
}

func (s *CardCreateConfirmWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.cards = imocks.NewCardManager(s.T())
	s.idem = &fakeCardCreateIdempotentWriter{}
	s.engine = workflow.NewEngine[CardCreateState](s.store, fake.NewProvider())
	s.def = BuildCardCreateConfirmWorkflow(s.idem, s.cards)
	s.userID = uuid.New()
	s.key = CardCreateKey(s.userID.String())
}

func (s *CardCreateConfirmWorkflowSuite) baseState() CardCreateState {
	return CardCreateState{
		Status:    CardCreateStatusActive,
		UserID:    s.userID,
		Nickname:  "Nubank",
		Bank:      "nubank",
		DueDay:    10,
		MessageID: "wamid-start",
	}
}

func (s *CardCreateConfirmWorkflowSuite) resume(text, incomingMessageID string) workflow.RunResult[CardCreateState] {
	result, _ := s.resumeWithErr(text, incomingMessageID)
	return result
}

func (s *CardCreateConfirmWorkflowSuite) resumeWithErr(text, incomingMessageID string) (workflow.RunResult[CardCreateState], error) {
	resumeBytes, err := json.Marshal(map[string]string{
		"resumeText":        text,
		"incomingMessageId": incomingMessageID,
	})
	s.Require().NoError(err)

	return s.engine.Resume(s.ctx, s.def, s.key, resumeBytes)
}

func (s *CardCreateConfirmWorkflowSuite) TestSuspend_PersistsStateBeforeAsking() {
	result, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.NotEmpty(result.State.ResponseText)
	s.Contains(result.State.ResponseText, "Nubank")
	s.Contains(result.State.ResponseText, "💳")

	snap, ok, err := s.store.Load(s.ctx, CardCreateConfirmWorkflowID, s.key)
	s.Require().NoError(err)
	s.True(ok)
	s.Equal(workflow.RunStatusSuspended, snap.Status)
}

func (s *CardCreateConfirmWorkflowSuite) TestAccept_InvokesWriteFnViaIdempotentExecute() {
	cardID := uuid.New()
	s.cards.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{ID: cardID.String(), Nickname: "Nubank"}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{invoke: true, outcome: agent.ToolOutcomeRouted},
	}

	result := s.resume("sim", "wamid-1")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(CardCreateStatusCompleted, result.State.Status)
	s.Contains(result.State.ResponseText, "✅")
	s.Contains(result.State.ResponseText, "💳")
	s.Contains(result.State.ResponseText, "cadastrado com sucesso")
	s.Equal(1, s.idem.calls)
}

func (s *CardCreateConfirmWorkflowSuite) TestAccept_IdempotentReplay_MessageWithoutEmoji() {
	cardID := uuid.New()
	s.cards.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{ID: cardID.String(), Nickname: "Nubank"}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{invoke: true, outcome: agent.ToolOutcomeReplay},
	}

	result := s.resume("sim", "wamid-1")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(CardCreateStatusCompleted, result.State.Status)
	s.Contains(result.State.ResponseText, "já estava cadastrado")
	s.NotContains(result.State.ResponseText, "💳")
}

func (s *CardCreateConfirmWorkflowSuite) TestAccept_NicknameConflict_DomainMessage_RunConcluded() {
	s.cards.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{}, carddomain.ErrNicknameConflict).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{invoke: true},
	}

	result := s.resume("sim", "wamid-1")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(CardCreateStatusCompleted, result.State.Status)
	s.Contains(result.State.ResponseText, "apelido")
	s.NotContains(result.State.ResponseText, "💳")
}

func (s *CardCreateConfirmWorkflowSuite) TestAccept_InvalidDueDay_ActionableRangeMessage_RunConcluded() {
	s.cards.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{}, carddomain.ErrInvalidDueDay).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{invoke: true},
	}

	result := s.resume("sim", "wamid-1")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(CardCreateStatusCompleted, result.State.Status)
	s.Contains(result.State.ResponseText, "31")
	s.Contains(result.State.ResponseText, "entre 1 e 31")
	s.NotContains(result.State.ResponseText, "💳")
}

func (s *CardCreateConfirmWorkflowSuite) TestAccept_InfraError_RunFailed_ErrorPersisted() {
	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{err: errCardCreateInfraStub},
		{err: errCardCreateInfraStub},
	}

	result, resumeErr := s.resumeWithErr("sim", "wamid-1")
	s.Error(resumeErr)
	s.Equal(workflow.RunStatusFailed, result.Status)

	snap, ok, err := s.store.Load(s.ctx, CardCreateConfirmWorkflowID, s.key)
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Equal(workflow.RunStatusFailed, snap.Status)
	s.NotEmpty(snap.LastError)
}

func (s *CardCreateConfirmWorkflowSuite) TestExpire_TTL_HandledFalse() {
	state := s.baseState()
	state.SuspendedAt = time.Now().UTC().Add(-16 * time.Minute)
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	result := s.resume("sim", "wamid-1")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.True(result.State.Expired)
	s.Empty(result.State.ResponseText)
}

func (s *CardCreateConfirmWorkflowSuite) TestReprompt_FirstAmbiguous_ResuspendsThenCancels() {
	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	result1 := s.resume("talvez", "wamid-1")
	s.Equal(workflow.RunStatusSuspended, result1.Status)
	s.Contains(result1.State.ResponseText, "sim")
	s.NotContains(result1.State.ResponseText, "💳")

	result2 := s.resume("quem sabe", "wamid-2")
	s.Equal(workflow.RunStatusSucceeded, result2.Status)
	s.Equal(CardCreateStatusCancelled, result2.State.Status)
	s.NotContains(result2.State.ResponseText, "💳")
}

func (s *CardCreateConfirmWorkflowSuite) TestCancel_Explicit_RunConcluded() {
	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	result := s.resume("não", "wamid-1")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(CardCreateStatusCancelled, result.State.Status)
	s.Contains(result.State.ResponseText, "cancelado")
	s.NotContains(result.State.ResponseText, "💳")
}

func (s *CardCreateConfirmWorkflowSuite) TestNoDecisionLeavesRunSuspended_Accept() {
	cardID := uuid.New()
	s.cards.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{ID: cardID.String(), Nickname: "Nubank"}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{invoke: true, outcome: agent.ToolOutcomeRouted},
	}
	result := s.resume("sim", "wamid-1")
	s.NotEqual(workflow.RunStatusSuspended, result.Status)
}

func (s *CardCreateConfirmWorkflowSuite) TestNoDecisionLeavesRunSuspended_Cancel() {
	_, err := s.engine.Start(s.ctx, s.def, s.key, s.baseState())
	s.Require().NoError(err)

	result := s.resume("não", "wamid-1")
	s.NotEqual(workflow.RunStatusSuspended, result.Status)
}

func (s *CardCreateConfirmWorkflowSuite) TestNoDecisionLeavesRunSuspended_Expire() {
	state := s.baseState()
	state.SuspendedAt = time.Now().UTC().Add(-16 * time.Minute)
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	result := s.resume("sim", "wamid-1")
	s.NotEqual(workflow.RunStatusSuspended, result.Status)
}
