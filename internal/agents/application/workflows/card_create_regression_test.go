package workflows

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type CardCreateRegressionSuite struct {
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

func TestCardCreateRegressionSuite(t *testing.T) {
	suite.Run(t, new(CardCreateRegressionSuite))
}

func (s *CardCreateRegressionSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.cards = imocks.NewCardManager(s.T())
	s.idem = &fakeCardCreateIdempotentWriter{}
	s.engine = workflow.NewEngine[CardCreateState](s.store, fake.NewProvider())
	s.def = BuildCardCreateConfirmWorkflow(s.idem, s.cards)
	s.userID = uuid.New()
	s.key = CardCreateKey(s.userID.String())
}

func (s *CardCreateRegressionSuite) TestNoRunStarted_ResumeIsNoop_NeverAffirmsSuccessOrFailure() {
	patch := []byte(`{"resumeText":"sim","incomingMessageId":"wamid-never-started"}`)
	result, err := s.engine.Resume(s.ctx, s.def, s.key, patch)

	s.Require().NoError(err, "RF-13: sem run suspenso, resume nunca deve propagar erro fabricado")
	s.Empty(result.State.ResponseText, "RF-13: sem tool call create_card, nenhuma mensagem de sucesso/falha pode ser produzida")
	s.NotContains(result.State.ResponseText, "cadastrei")
	s.NotContains(result.State.ResponseText, "não consegui")
	s.NotContains(result.State.ResponseText, "Não consegui")
}

func (s *CardCreateRegressionSuite) TestInfraFailure_ErrorNeverEmpty_RunFailedPersisted() {
	_, err := s.engine.Start(s.ctx, s.def, s.key, CardCreateState{
		Status:    CardCreateStatusActive,
		UserID:    s.userID,
		Nickname:  "Nubank",
		Bank:      "nubank",
		DueDay:    10,
		MessageID: "wamid-start",
	})
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{
		{err: errCardCreateInfraStub},
		{err: errCardCreateInfraStub},
	}

	resumeBytes := []byte(`{"resumeText":"sim","incomingMessageId":"wamid-1"}`)
	result, resumeErr := s.engine.Resume(s.ctx, s.def, s.key, resumeBytes)

	s.Require().Error(resumeErr, "RF-15: falha de infraestrutura no cadastro nunca deve ser engolida")
	s.NotEmpty(resumeErr.Error(), "RF-15: o erro retornado pelo resume nunca é vazio")
	s.Equal(workflow.RunStatusFailed, result.Status)

	snap, ok, loadErr := s.store.Load(s.ctx, CardCreateConfirmWorkflowID, s.key)
	s.Require().NoError(loadErr)
	s.Require().True(ok)
	s.Equal(workflow.RunStatusFailed, snap.Status)
	s.NotEmpty(snap.LastError, "RF-15: run.error (mecanismo do kernel) deve estar preenchido, nunca vazio")
}

func (s *CardCreateRegressionSuite) TestNicknameConflict_IsDomainOutcome_NotSilentFailure() {
	s.cards.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{}, carddomain.ErrNicknameConflict).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, s.key, CardCreateState{
		Status:    CardCreateStatusActive,
		UserID:    s.userID,
		Nickname:  "Nubank",
		Bank:      "nubank",
		DueDay:    10,
		MessageID: "wamid-start",
	})
	s.Require().NoError(err)

	s.idem.results = []cardCreateIdempotentResult{{invoke: true}}

	resumeBytes := []byte(`{"resumeText":"sim","incomingMessageId":"wamid-1"}`)
	result, resumeErr := s.engine.Resume(s.ctx, s.def, s.key, resumeBytes)

	s.Require().NoError(resumeErr, "conflito de apelido é outcome de domínio, não falha de infra silenciosa")
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.NotEmpty(result.State.ResponseText)
	s.Contains(result.State.ResponseText, "apelido")
}
