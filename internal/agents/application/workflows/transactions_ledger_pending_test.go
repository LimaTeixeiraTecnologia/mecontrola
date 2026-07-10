package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type TransactionsLedgerPendingSuite struct {
	suite.Suite
	ctx    context.Context
	store  *wfStore
	engine workflow.Engine[PendingEntryState]
	ledger *imocks.TransactionsLedger
	userID uuid.UUID
}

func TestTransactionsLedgerPendingSuite(t *testing.T) {
	suite.Run(t, new(TransactionsLedgerPendingSuite))
}

func (s *TransactionsLedgerPendingSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.engine = workflow.NewEngine[PendingEntryState](s.store, fake.NewProvider())
	s.userID = uuid.New()
}

func (s *TransactionsLedgerPendingSuite) buildDef() workflow.Definition[PendingEntryState] {
	return BuildPendingEntryWorkflow(s.ledger, nil, nil, nil)
}

func (s *TransactionsLedgerPendingSuite) key(suffix string) string {
	return PendingEntryKey(s.userID.String(), suffix)
}

func (s *TransactionsLedgerPendingSuite) resumeWith(text string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text})
	return b
}

func (s *TransactionsLedgerPendingSuite) resumeWithMsgID(text, msgID string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text, "incomingMessageId": msgID})
	return b
}

func (s *TransactionsLedgerPendingSuite) baseState(opKind PendingOperationKind) PendingEntryState {
	rootID := uuid.New()
	subID := uuid.New()
	return PendingEntryState{
		Status:          PendingStatusActive,
		Awaiting:        AwaitingSlotConfirmation,
		OperationKind:   opKind,
		UserID:          s.userID,
		ResourceID:      s.userID,
		ThreadID:        "thr-ledger-001",
		MessageID:       "wamid-original-001",
		OriginalText:    "Gastei R$ 150,00 no supermercado hoje no pix",
		AmountCents:     15000,
		Description:     "supermercado",
		PaymentMethod:   "pix",
		OccurredAt:      "2026-07-06",
		CategoryVersion: 1,
		Kind:            ifaces.CategoryKindExpense,
		Candidates: []PendingCategoryCandidate{
			{
				RootCategoryID:  rootID,
				RootSlug:        "custo-fixo",
				SubcategoryID:   subID,
				SubcategorySlug: "supermercado",
				Path:            "Custo Fixo > Supermercado",
			},
		},
	}
}

func (s *TransactionsLedgerPendingSuite) TestWriteOk_CreateTransaction_G7_20() {
	state := s.baseState(PendingOpRegisterExpense)
	k := s.key("thr-write-ok")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.MatchedBy(func(in ifaces.RawTransaction) bool {
			return in.OriginWamid == "wamid-original-001" &&
				in.OriginOperation == originOperationPending &&
				in.CategorySource == categorySrcUserSelected &&
				in.AmountCents == 15000 &&
				in.PaymentMethod == "pix" &&
				in.Direction == "outcome"
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCompleted, result.State.Status)
	s.Contains(result.State.ResponseText, "R$ 150,00")
	s.NotContains(result.State.ResponseText, "Não consegui registrar")
}

func (s *TransactionsLedgerPendingSuite) TestWriteOk_Income_CreateTransaction() {
	state := s.baseState(PendingOpRegisterIncome)
	state.Kind = ifaces.CategoryKindIncome
	state.Description = "salário"
	k := s.key("thr-write-income")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.MatchedBy(func(in ifaces.RawTransaction) bool {
			return in.Direction == "income"
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.NoError(err)
	s.Equal(PendingStatusCompleted, result.State.Status)
}

func (s *TransactionsLedgerPendingSuite) TestReplayIdempotent_SameMsgID_NoSecondWrite_G7_09_CA07() {
	state := s.baseState(PendingOpRegisterExpense)
	state.MessageID = "wamid-replay-same"
	k := s.key("thr-replay-same-msgid")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	first, err := s.engine.Resume(s.ctx, def, k, s.resumeWithMsgID("sim", "wamid-confirm-1"))
	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, first.Status)
	s.Equal(PendingStatusCompleted, first.State.Status)

	second, err := s.engine.Resume(s.ctx, def, k, s.resumeWithMsgID("sim", "wamid-confirm-1"))
	s.NoError(err)
	s.Equal(workflow.RunStatus(0), second.Status)
}

func (s *TransactionsLedgerPendingSuite) TestLedgerError_NoSuccessResponse_G7_15_CA06_M03() {
	state := s.baseState(PendingOpRegisterExpense)
	k := s.key("thr-ledger-error")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{}, errors.New("ledger 500")).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.Error(err, "RF-10: real write failure must propagate as error, not be swallowed")
	s.Contains(err.Error(), "ledger 500", "RF-10: real error reason must be propagated, not swallowed")
	s.Equal(workflow.RunStatusFailed, result.Status)
	s.NotEqual(PendingStatusCompleted, result.State.Status)
	s.NotContains(result.State.ResponseText, "registrei")
	s.NotContains(result.State.ResponseText, "anotei")
	s.NotContains(result.State.ResponseText, "salvo")
}

func (s *TransactionsLedgerPendingSuite) TestLedgerNilID_NoSuccessResponse_M03() {
	state := s.baseState(PendingOpRegisterExpense)
	k := s.key("thr-ledger-nil-id")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.Nil, Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.Error(err)
	s.ErrorIs(err, ErrWriteAcceptedWithoutResource)
	s.Equal(workflow.RunStatusFailed, result.Status)
	s.Equal(PendingStatusActive, result.State.Status)
	s.NotEqual(PendingStatusCancelled, result.State.Status)
	s.NotContains(result.State.ResponseText, "registrei")
}

func (s *TransactionsLedgerPendingSuite) TestEditEntry_UpdateTransaction_TargetVersionPreserved_CA17() {
	targetID := uuid.New()
	state := s.baseState(PendingOpEditEntry)
	state.TargetTransactionID = &targetID
	state.TargetVersion = 3
	k := s.key("thr-edit-update")
	def := s.buildDef()

	s.ledger.EXPECT().
		UpdateTransaction(mock.Anything, mock.MatchedBy(func(in ifaces.RawUpdateTransaction) bool {
			return in.ID == targetID &&
				in.Version == 3 &&
				in.CategorySource == categorySrcUserSelected &&
				in.AmountCents == 15000
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.NoError(err)
	s.Equal(PendingStatusCompleted, result.State.Status)
	s.Contains(result.State.ResponseText, "R$ 150,00")
}

func (s *TransactionsLedgerPendingSuite) TestEditEntry_NoCreateTransactionCalled() {
	targetID := uuid.New()
	state := s.baseState(PendingOpEditEntry)
	state.TargetTransactionID = &targetID
	state.TargetVersion = 1
	k := s.key("thr-edit-no-create")
	def := s.buildDef()

	s.ledger.EXPECT().
		UpdateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.New()}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.NoError(err)
	s.Equal(PendingStatusCompleted, result.State.Status)
}

func (s *TransactionsLedgerPendingSuite) TestRecurrenceConfirmed_CreateRecurringTemplate_G9_01_CA16() {
	state := s.baseState(PendingOpCreateRecurrence)
	state.Frequency = "monthly"
	state.RecurrenceDayOfMonth = 1
	k := s.key("thr-recurrence-ok")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateRecurringTemplate(mock.Anything, mock.MatchedBy(func(in ifaces.RawRecurringTemplate) bool {
			return in.Frequency == "monthly" &&
				in.DayOfMonth == 1 &&
				in.AmountCents == 15000 &&
				in.CategorySource == categorySrcUserSelected &&
				in.OriginWamid == "wamid-original-001"
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindRecurringTemplate}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.NoError(err)
	s.Equal(PendingStatusCompleted, result.State.Status)
	s.NotContains(result.State.ResponseText, "Não consegui registrar")
}

func (s *TransactionsLedgerPendingSuite) TestRecurrenceCancelled_NoWrite_G9_02() {
	state := s.baseState(PendingOpCreateRecurrence)
	state.Frequency = "monthly"
	state.RecurrenceDayOfMonth = 1
	k := s.key("thr-recurrence-cancel")
	def := s.buildDef()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("não"))

	s.NoError(err)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *TransactionsLedgerPendingSuite) TestRootWithoutLeaf_BlockedBeforeLedger_G10_01_M04() {
	state := s.baseState(PendingOpRegisterExpense)
	rootID := uuid.New()
	state.Candidates = []PendingCategoryCandidate{
		{
			RootCategoryID:  rootID,
			SubcategoryID:   uuid.UUID{},
			SubcategorySlug: "",
			Path:            "Vendas",
		},
	}
	k := s.key("thr-root-no-leaf")
	def := s.buildDef()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.NoError(err)
	s.Equal(PendingStatusCancelled, result.State.Status)
	s.Contains(result.State.ResponseText, "categoria")
	s.NotContains(result.State.ResponseText, "registrei")
	s.NotContains(result.State.ResponseText, "anotei")
}

func (s *TransactionsLedgerPendingSuite) TestConfirmationUniversal_NoWriteWithoutSim_M07() {
	state := s.baseState(PendingOpRegisterExpense)
	state.Awaiting = AwaitingSlotCategory
	k := s.key("thr-no-write-without-confirm")
	def := s.buildDef()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("cancela"))

	s.NoError(err)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *TransactionsLedgerPendingSuite) TestSuccessText_ContainsNoSuccessSimulated_G10_03() {
	state := s.baseState(PendingOpRegisterExpense)
	k := s.key("thr-no-sim-success")
	def := s.buildDef()

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{}, errors.New("ledger error")).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumeWith("sim"))

	s.Error(err, "RF-10: real write failure must propagate as error, not be swallowed")
	s.NotContains(result.State.ResponseText, "registrei")
	s.NotContains(result.State.ResponseText, "anotei")
	s.NotContains(result.State.ResponseText, "salvo")
}
