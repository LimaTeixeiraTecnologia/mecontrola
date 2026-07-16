package workflows

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

func txPaymentMethodMigrationErr() error {
	return txusecases.ErrPaymentMethodMigrationNotAllowed
}

type TransactionWriteWorkflowSuite struct {
	suite.Suite
	ctx    context.Context
	store  *wfStore
	engine workflow.Engine[TransactionWriteState]
	def    workflow.Definition[TransactionWriteState]
	ledger *imocks.TransactionsLedger
	userID uuid.UUID
}

func TestTransactionWriteWorkflowSuite(t *testing.T) {
	suite.Run(t, new(TransactionWriteWorkflowSuite))
}

func (s *TransactionWriteWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.engine = workflow.NewEngine[TransactionWriteState](s.store, fake.NewProvider())
	s.def = BuildTransactionWriteWorkflowWithObservability(s.ledger, nil, nil, nil, nil)
	s.userID = uuid.New()
}

func (s *TransactionWriteWorkflowSuite) key(suffix string) string {
	return TransactionWriteKey(s.userID.String(), suffix)
}

func (s *TransactionWriteWorkflowSuite) resumePayload(text string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text})
	return b
}

func (s *TransactionWriteWorkflowSuite) resumePayloadWithMsgID(text, msgID string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text, "incomingMessageId": msgID})
	return b
}

func (s *TransactionWriteWorkflowSuite) newExpenseState() TransactionWriteState {
	return TransactionWriteState{
		Status:        TransactionWriteStatusActive,
		Awaiting:      TransactionAwaitingConfirmation,
		OperationKind: TransactionOpRegisterExpense,
		UserID:        s.userID,
		ResourceID:    s.userID,
		ThreadID:      "thr-001",
		MessageID:     "wamid-001",
		AmountCents:   15000,
		Description:   "mercado",
		PaymentMethod: "pix",
		Candidates: []PendingCategoryCandidate{{
			RootCategoryID:  uuid.New(),
			RootSlug:        "custo-fixo",
			SubcategoryID:   uuid.New(),
			SubcategorySlug: "mercado",
			Path:            "Custo Fixo > Mercado",
		}},
		CategoryVersion: 1,
	}
}

func (s *TransactionWriteWorkflowSuite) TestStart_SuspendsForConfirmation() {
	state := s.newExpenseState()
	k := s.key("thr-confirm-suspend")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Contains(result.State.ResponseText, "Posso registrar?")
}

func (s *TransactionWriteWorkflowSuite) TestResume_Confirmation_Accept_DirectWrite_Success() {
	state := s.newExpenseState()
	k := s.key("thr-confirm-accept")

	txID := uuid.New()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: txID, Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCompleted, result.State.Status)
	s.Equal(txID, result.State.ResourceID)
	s.NotEmpty(result.State.ResponseText)
}

func (s *TransactionWriteWorkflowSuite) TestResume_Confirmation_Cancel() {
	state := s.newExpenseState()
	k := s.key("thr-confirm-cancel")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("não"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCancelled, result.State.Status)
	s.ledger.AssertNotCalled(s.T(), "CreateTransaction", mock.Anything, mock.Anything)
}

func (s *TransactionWriteWorkflowSuite) TestResume_Confirmation_WithoutResolvedCategory_CancelsWithoutWriting() {
	state := s.newExpenseState()
	state.Candidates = nil
	k := s.key("thr-confirm-no-category")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCancelled, result.State.Status)
	s.ledger.AssertNotCalled(s.T(), "CreateTransaction", mock.Anything, mock.Anything)
}

func (s *TransactionWriteWorkflowSuite) TestResume_Confirmation_AmbiguousThenCancel() {
	state := s.newExpenseState()
	k := s.key("thr-confirm-ambiguous")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("talvez"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)

	result, err = s.engine.Resume(s.ctx, s.def, k, s.resumePayload("talvez"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCancelled, result.State.Status)
}

func (s *TransactionWriteWorkflowSuite) TestResume_Confirmation_FalsoSucessoGuard() {
	state := s.newExpenseState()
	k := s.key("thr-false-success")

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.Error(err)
	s.ErrorIs(err, ErrTransactionWriteAcceptedWithoutResource)
	s.Equal(workflow.RunStatusFailed, result.Status)
	s.NotContains(result.State.ResponseText, "Prontinho")
}

func (s *TransactionWriteWorkflowSuite) TestResume_Confirmation_Replay_NoDuplicateWrite() {
	state := s.newExpenseState()
	k := s.key("thr-replay")

	txID := uuid.New()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: txID, Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	firstResult, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayloadWithMsgID("sim", "wamid-dup"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, firstResult.Status)

	secondResult, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayloadWithMsgID("sim", "wamid-dup"))
	s.Require().NoError(err)
	s.Zero(secondResult.Status)
	s.ledger.AssertNumberOfCalls(s.T(), "CreateTransaction", 1)
}

func (s *TransactionWriteWorkflowSuite) TestStart_PaymentMethodMissing_AsksSlot() {
	state := s.newExpenseState()
	state.Awaiting = 0
	state.PaymentMethod = ""
	k := s.key("thr-payment-slot")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(TransactionAwaitingPaymentMethod, result.State.Awaiting)
}

func (s *TransactionWriteWorkflowSuite) TestResume_PaymentMethodSlot_FillsThenAsksConfirmation() {
	state := s.newExpenseState()
	state.Awaiting = TransactionAwaitingPaymentMethod
	state.PaymentMethod = ""
	k := s.key("thr-payment-fill")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("pix"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(TransactionAwaitingConfirmation, result.State.Awaiting)
	s.Equal("pix", result.State.PaymentMethod)
}

func (s *TransactionWriteWorkflowSuite) TestEditEntry_SingleCandidate_PromotesToConfirmation() {
	target := uuid.New()
	catID := uuid.New()

	s.ledger.EXPECT().
		SearchEditCandidates(mock.Anything, s.userID, mock.Anything).
		Return([]ifaces.Entry{{
			Kind:                 ifaces.EntryKindTransaction,
			ID:                   target.String(),
			AmountCents:          9000,
			Description:          "mercado",
			CategoryID:           catID.String(),
			CategoryNameSnapshot: "Mercado",
			PaymentMethod:        "pix",
			Version:              1,
		}}, nil).
		Once()

	state := TransactionWriteState{
		Status:                TransactionWriteStatusActive,
		OperationKind:         TransactionOpEditEntry,
		UserID:                s.userID,
		ResourceID:            s.userID,
		ThreadID:              "thr-edit",
		MessageID:             "wamid-edit-1",
		AmountCents:           9500,
		Kind:                  ifaces.CategoryKindExpense,
		EditSearchAmountCents: 9000,
	}
	k := s.key("thr-edit-single")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(TransactionAwaitingConfirmation, result.State.Awaiting)
	s.NotNil(result.State.TargetTransactionID)
	s.Equal(target, *result.State.TargetTransactionID)
	s.Contains(result.State.ResponseText, "Posso atualizar?")
}

func (s *TransactionWriteWorkflowSuite) TestEditEntry_NoCandidates_Cancels() {
	s.ledger.EXPECT().
		SearchEditCandidates(mock.Anything, s.userID, mock.Anything).
		Return(nil, nil).
		Once()

	state := TransactionWriteState{
		Status:                TransactionWriteStatusActive,
		OperationKind:         TransactionOpEditEntry,
		UserID:                s.userID,
		ResourceID:            s.userID,
		ThreadID:              "thr-edit-empty",
		MessageID:             "wamid-edit-2",
		EditSearchAmountCents: 500,
	}
	k := s.key("thr-edit-none")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCancelled, result.State.Status)
}

func (s *TransactionWriteWorkflowSuite) TestEditEntry_MultipleCandidates_ThenChooseAndConfirm() {
	target1 := uuid.New()
	target2 := uuid.New()
	catID := uuid.New()

	s.ledger.EXPECT().
		SearchEditCandidates(mock.Anything, s.userID, mock.Anything).
		Return([]ifaces.Entry{
			{Kind: ifaces.EntryKindTransaction, ID: target1.String(), AmountCents: 3000, CategoryID: catID.String(), CategoryNameSnapshot: "Transporte", PaymentMethod: "pix", Version: 1},
			{Kind: ifaces.EntryKindTransaction, ID: target2.String(), AmountCents: 3000, CategoryID: catID.String(), CategoryNameSnapshot: "Lazer", PaymentMethod: "cash", Version: 1},
		}, nil).
		Once()

	updated := uuid.New()
	s.ledger.EXPECT().
		UpdateTransaction(mock.Anything, mock.MatchedBy(func(in ifaces.RawUpdateTransaction) bool {
			return in.ID == target2
		})).
		Return(ifaces.EntryRef{ID: updated, Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	state := TransactionWriteState{
		Status:                TransactionWriteStatusActive,
		OperationKind:         TransactionOpEditEntry,
		UserID:                s.userID,
		ResourceID:            s.userID,
		ThreadID:              "thr-edit-multi",
		MessageID:             "wamid-edit-3",
		Kind:                  ifaces.CategoryKindExpense,
		EditSearchAmountCents: 3000,
	}
	k := s.key("thr-edit-multi")

	result, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(TransactionAwaitingEditCandidate, result.State.Awaiting)

	result, err = s.engine.Resume(s.ctx, s.def, k, s.resumePayload("2"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(TransactionAwaitingConfirmation, result.State.Awaiting)

	result, err = s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCompleted, result.State.Status)
}

func (s *TransactionWriteWorkflowSuite) TestEditEntry_PaymentMethodMigrationGuard_BlocksAndCancels() {
	target := uuid.New()
	catID := uuid.New()

	s.ledger.EXPECT().
		SearchEditCandidates(mock.Anything, s.userID, mock.Anything).
		Return([]ifaces.Entry{{
			Kind:                 ifaces.EntryKindTransaction,
			ID:                   target.String(),
			AmountCents:          9000,
			Description:          "mercado",
			CategoryID:           catID.String(),
			CategoryNameSnapshot: "Mercado",
			PaymentMethod:        "pix",
			Version:              1,
		}}, nil).
		Once()

	s.ledger.EXPECT().
		UpdateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{}, txPaymentMethodMigrationErr()).
		Once()

	state := TransactionWriteState{
		Status:                TransactionWriteStatusActive,
		OperationKind:         TransactionOpEditEntry,
		UserID:                s.userID,
		ResourceID:            s.userID,
		ThreadID:              "thr-edit-guard",
		MessageID:             "wamid-edit-4",
		Kind:                  ifaces.CategoryKindExpense,
		PaymentMethod:         "credit_card",
		EditSearchAmountCents: 9000,
	}
	k := s.key("thr-edit-guard")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCancelled, result.State.Status)
	s.Contains(result.State.ResponseText, "não pode migrar")
}

func (s *TransactionWriteWorkflowSuite) TestResume_Expired() {
	state := s.newExpenseState()
	k := s.key("thr-expired")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	snap, _, _ := s.store.Load(s.ctx, TransactionWriteWorkflowID, k)
	var loaded TransactionWriteState
	_ = json.Unmarshal(snap.State, &loaded)
	loaded.SuspendedAt = loaded.SuspendedAt.Add(-time.Hour)
	raw, _ := json.Marshal(loaded)
	snap.State = raw
	s.store.data[s.store.key(TransactionWriteWorkflowID, k)] = snap

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusExpired, result.State.Status)
}

func (s *TransactionWriteWorkflowSuite) TestRecurrence_SuspendsWithRecurrenceBlock_ThenSucceeds() {
	state := TransactionWriteState{
		Status:        TransactionWriteStatusActive,
		Awaiting:      TransactionAwaitingConfirmation,
		OperationKind: TransactionOpCreateRecurrence,
		UserID:        s.userID,
		ResourceID:    s.userID,
		ThreadID:      "thr-recurrence",
		MessageID:     "wamid-recurrence-1",
		AmountCents:   5000,
		Description:   "academia",
		PaymentMethod: "pix",
		Frequency:     "monthly",
		Candidates: []PendingCategoryCandidate{{
			RootCategoryID: uuid.New(),
			SubcategoryID:  uuid.New(),
			Path:           "Saúde > Academia",
		}},
	}
	k := s.key("thr-recurrence")

	created := uuid.New()
	s.ledger.EXPECT().
		CreateRecurringTemplate(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: created, Kind: ifaces.EntryKindRecurringTemplate}, nil).
		Once()

	start, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, start.Status)
	s.Contains(start.State.ResponseText, "Posso configurar?")

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(TransactionWriteStatusCompleted, result.State.Status)
}
