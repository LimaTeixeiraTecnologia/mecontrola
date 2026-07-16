package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DestructiveManageWorkflowSuite struct {
	suite.Suite
	ctx             context.Context
	cardsMock       *interfacemocks.CardManager
	recurrencesMock *interfacemocks.RecurrenceManager
	ledgerMock      *interfacemocks.TransactionsLedger
	userID          uuid.UUID
	cardID          uuid.UUID
}

func TestDestructiveManageWorkflowSuite(t *testing.T) {
	suite.Run(t, new(DestructiveManageWorkflowSuite))
}

func (s *DestructiveManageWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.cardsMock = interfacemocks.NewCardManager(s.T())
	s.recurrencesMock = interfacemocks.NewRecurrenceManager(s.T())
	s.ledgerMock = interfacemocks.NewTransactionsLedger(s.T())
	s.userID = uuid.New()
	s.cardID = uuid.New()
}

func (s *DestructiveManageWorkflowSuite) TestBuildDestructiveManageWorkflow_Definition() {
	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	s.Equal(DestructiveManageWorkflowID, def.ID)
	s.True(def.Durable)
	s.Equal(1, def.MaxAttempts)
	s.NotNil(def.Root)
}

func (s *DestructiveManageWorkflowSuite) TestFirstEntrySuspendsWithImpactNote() {
	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:     s.userID,
		Operation:  DestructiveOpDeleteCard,
		TargetRef:  s.cardID.String(),
		ImpactNote: "Remoção permanente do 💳.",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Contains(out.Suspend.Prompt, "Remoção permanente")
}

func (s *DestructiveManageWorkflowSuite) TestConfirmDeleteCard() {
	s.cardsMock.EXPECT().
		SoftDeleteCard(mock.Anything, s.cardID, s.userID).
		Return(nil).Once()

	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:      s.userID,
		Operation:   DestructiveOpDeleteCard,
		TargetRef:   s.cardID.String(),
		ImpactNote:  "Remoção permanente do 💳.",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "sim",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(DestructiveManageCompleted, out.State.Status)
	s.Contains(out.State.ResponseText, "removido com sucesso")
}

func (s *DestructiveManageWorkflowSuite) TestConfirmDeleteRecurrence() {
	s.recurrencesMock.EXPECT().
		DeleteRecurrence(mock.Anything, "template-1", int64(3)).
		Return(nil).Once()

	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:      s.userID,
		Operation:   DestructiveOpDeleteRecurrence,
		TargetRef:   "template-1",
		Version:     3,
		ImpactNote:  "Esta recorrência será removida permanentemente.",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "sim",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "Recorrência removida")
}

func (s *DestructiveManageWorkflowSuite) TestConfirmDeleteEntry() {
	entryID := uuid.New()
	s.ledgerMock.EXPECT().
		DeleteTransaction(mock.Anything, interfaces.EntryRef{ID: entryID}, int64(2)).
		Return(nil).Once()

	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:      s.userID,
		Operation:   DestructiveOpDeleteEntry,
		TargetRef:   entryID.String(),
		Version:     2,
		ImpactNote:  "Este lançamento será removido permanentemente.",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "sim",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "Lançamento removido")
}

func (s *DestructiveManageWorkflowSuite) TestConfirmUpdateRecurrence() {
	s.recurrencesMock.EXPECT().
		UpdateRecurrence(mock.Anything, "template-2", mock.Anything).
		Return(interfaces.EntryRef{}, nil).Once()

	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:        s.userID,
		Operation:     DestructiveOpUpdateRecurrence,
		TargetRef:     "template-2",
		Version:       4,
		ImpactNote:    "Esta recorrência será atualizada.",
		SuspendedAt:   time.Now().UTC(),
		ResumeText:    "sim",
		UpdatePayload: `{"amountCents":5000}`,
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Contains(out.State.ResponseText, "Recorrência atualizada")
}

func (s *DestructiveManageWorkflowSuite) TestConfirmCancel() {
	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:      s.userID,
		Operation:   DestructiveOpDeleteCard,
		TargetRef:   s.cardID.String(),
		ImpactNote:  "Remoção permanente do 💳.",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "não",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(DestructiveManageCancelled, out.State.Status)
}

func (s *DestructiveManageWorkflowSuite) TestConfirmExpire() {
	def := BuildDestructiveManageWorkflow(s.cardsMock, s.recurrencesMock, s.ledgerMock)
	state := DestructiveManageState{
		UserID:      s.userID,
		Operation:   DestructiveOpDeleteCard,
		TargetRef:   s.cardID.String(),
		ImpactNote:  "Remoção permanente do 💳.",
		SuspendedAt: time.Now().UTC().Add(-6 * time.Minute),
		ResumeText:  "sim",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.True(out.State.Expired)
}

func (s *DestructiveManageWorkflowSuite) TestBuildDestructiveManageImpactNoteCardWithOpenInstallments() {
	s.cardsMock.EXPECT().
		HasOpenInstallments(mock.Anything, s.cardID, s.userID).
		Return(true, nil).Once()

	note := BuildDestructiveManageImpactNote(s.ctx, s.cardID.String(), "card", s.userID, s.cardsMock)

	s.Contains(note, "parceladas em aberto")
}

func (s *DestructiveManageWorkflowSuite) TestBuildDestructiveManageReaper() {
	reaper := BuildDestructiveManageReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}
