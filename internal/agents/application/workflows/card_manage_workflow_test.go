package workflows

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeIdemWriter struct {
	seen map[string]uuid.UUID
}

func newFakeIdemWriter() *fakeIdemWriter {
	return &fakeIdemWriter{seen: map[string]uuid.UUID{}}
}

func (f *fakeIdemWriter) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write IdempotentWriteFn,
	isDomainErr DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	key := fmt.Sprintf("%s:%d:%s", wamid, itemSeq, operation)
	if id, ok := f.seen[key]; ok {
		return id, agent.ToolOutcomeReplay, nil
	}
	id, _, err := write(ctx)
	if err != nil {
		return uuid.Nil, agent.ToolOutcomeUsecaseError, err
	}
	f.seen[key] = id
	return id, agent.ToolOutcomeRouted, nil
}

type CardManageWorkflowSuite struct {
	suite.Suite
	ctx       context.Context
	cardsMock *interfacemocks.CardManager
	idem      *fakeIdemWriter
	userID    uuid.UUID
	cardID    uuid.UUID
}

func TestCardManageWorkflowSuite(t *testing.T) {
	suite.Run(t, new(CardManageWorkflowSuite))
}

func (s *CardManageWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.cardsMock = interfacemocks.NewCardManager(s.T())
	s.idem = newFakeIdemWriter()
	s.userID = uuid.New()
	s.cardID = uuid.New()
}

func (s *CardManageWorkflowSuite) TestBuildCardManageWorkflow_Definition() {
	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	s.Equal(CardManageWorkflowID, def.ID)
	s.True(def.Durable)
	s.Equal(1, def.MaxAttempts)
	s.NotNil(def.Root)
}

func (s *CardManageWorkflowSuite) TestCreateFirstEntrySuspendsWithConfirmQuestion() {
	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	state := CardManageState{
		UserID:    s.userID,
		Operation: CardManageOpCreate,
		Nickname:  "Nubank",
		Bank:      "nubank",
		DueDay:    10,
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Contains(out.Suspend.Prompt, "Nubank")
}

func (s *CardManageWorkflowSuite) TestCreateConfirmExecutesCreateCard() {
	state := CardManageState{
		UserID:      s.userID,
		Operation:   CardManageOpCreate,
		Nickname:    "Nubank",
		Bank:        "nubank",
		DueDay:      10,
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "sim",
	}

	s.cardsMock.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{ID: s.cardID.String(), Nickname: "Nubank"}, nil).Once()

	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(CardManageCompleted, out.State.Status)
	s.Contains(out.State.ResponseText, "cadastrado com sucesso")
}

func (s *CardManageWorkflowSuite) TestCreateConfirmIsIdempotentByWamid() {
	newState := func() CardManageState {
		return CardManageState{
			UserID:      s.userID,
			Operation:   CardManageOpCreate,
			Nickname:    "Nubank",
			Bank:        "nubank",
			DueDay:      10,
			MessageID:   "wamid-card-dup",
			SuspendedAt: time.Now().UTC(),
			ResumeText:  "sim",
		}
	}

	s.cardsMock.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{ID: s.cardID.String(), Nickname: "Nubank"}, nil).Once()

	def := BuildCardManageWorkflow(s.cardsMock, s.idem)

	first, err := def.Root.Execute(s.ctx, newState())
	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, first.Status)
	s.Contains(first.State.ResponseText, "cadastrado com sucesso")

	second, err := def.Root.Execute(s.ctx, newState())
	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, second.Status)
	s.Contains(second.State.ResponseText, "já estava cadastrado")
}

func (s *CardManageWorkflowSuite) TestCreateConfirmDomainErrorNicknameConflict() {
	state := CardManageState{
		UserID:      s.userID,
		Operation:   CardManageOpCreate,
		Nickname:    "Nubank",
		Bank:        "nubank",
		DueDay:      10,
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "sim",
	}

	s.cardsMock.EXPECT().
		CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Return(interfaces.CardRef{}, carddomain.ErrNicknameConflict).Once()

	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "apelido")
}

func (s *CardManageWorkflowSuite) TestEditEntryFetchesCurrentCardForPreview() {
	s.cardsMock.EXPECT().
		GetCard(mock.Anything, s.cardID, s.userID).
		Return(interfaces.Card{ID: s.cardID.String(), Nickname: "Nubank", Bank: "nubank", DueDay: 10}, nil).Once()

	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	state := CardManageState{
		UserID:           s.userID,
		Operation:        CardManageOpEdit,
		CardID:           s.cardID.String(),
		Nickname:         "Nubank Roxo",
		NicknameProvided: true,
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.True(out.State.PreviousFetched)
	s.Equal("Nubank", out.State.PreviousNickname)
	s.Contains(out.Suspend.Prompt, "Nubank Roxo")
}

func (s *CardManageWorkflowSuite) TestEditConfirmExecutesUpdateCard() {
	state := CardManageState{
		UserID:           s.userID,
		Operation:        CardManageOpEdit,
		CardID:           s.cardID.String(),
		Nickname:         "Nubank Roxo",
		NicknameProvided: true,
		PreviousFetched:  true,
		PreviousNickname: "Nubank",
		PreviousBank:     "nubank",
		PreviousDueDay:   10,
		SuspendedAt:      time.Now().UTC(),
		ResumeText:       "sim",
	}

	s.cardsMock.EXPECT().
		UpdateCard(mock.Anything, mock.AnythingOfType("interfaces.CardUpdate")).
		Return(interfaces.Card{}, nil).Once()

	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "atualizado com sucesso")
}

func (s *CardManageWorkflowSuite) TestConfirmCancel() {
	state := CardManageState{
		UserID:      s.userID,
		Operation:   CardManageOpCreate,
		Nickname:    "Nubank",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "não",
	}

	def := BuildCardManageWorkflow(s.cardsMock, s.idem)
	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(CardManageCancelled, out.State.Status)
}

func (s *CardManageWorkflowSuite) TestBuildCardManageReaper() {
	reaper := BuildCardManageReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}
