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

func (s *TransactionWriteWorkflowSuite) newRootListingState() TransactionWriteState {
	state := s.newExpenseState()
	state.Awaiting = TransactionAwaitingCategory
	state.PaymentMethod = ""
	state.Kind = ifaces.CategoryKindExpense
	state.Candidates = []PendingCategoryCandidate{
		{RootCategoryID: testRootCustoFixoID, RootSlug: "custo-fixo", Path: "Custo Fixo"},
		{RootCategoryID: testRootPrazeresID, RootSlug: "prazeres", Path: "Prazeres"},
	}
	return state
}

func (s *TransactionWriteWorkflowSuite) TestStart_RootListing_PromptsNumberedRoots() {
	state := s.newRootListingState()
	k := s.key("thr-root-listing")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Contains(result.State.ResponseText, "Em qual categoria isso se encaixa? 📂")
	s.Contains(result.State.ResponseText, "1. *Custo Fixo*")
	s.Contains(result.State.ResponseText, "2. *Prazeres*")
}

func (s *TransactionWriteWorkflowSuite) TestResume_RootChoice_ExpandsLeaves() {
	cats := imocks.NewCategoriesReader(s.T())
	cats.EXPECT().
		ListCategories(mock.Anything, s.userID).
		Return([]ifaces.Category{{
			ID: testRootCustoFixoID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense",
			Subcategories: []ifaces.Category{
				{ID: testLeafCombustivel, Slug: "combustivel", Name: "Combustível"},
				{ID: testLeafSupermercado, Slug: "supermercado", Name: "Supermercado"},
			},
		}}, nil).
		Once()
	cats.EXPECT().
		SearchDictionary(mock.Anything, mock.Anything, mock.Anything).
		Return(ifaces.CategorySearchResult{}, nil).
		Maybe()
	def := BuildTransactionWriteWorkflowWithObservability(s.ledger, nil, cats, nil, nil)

	state := s.newRootListingState()
	k := s.key("thr-root-expand")

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("1"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Contains(result.State.ResponseText, "Dentro de *Custo Fixo*, qual subcategoria? 📂")
	s.Contains(result.State.ResponseText, "1. *Combustível*")
	s.Contains(result.State.ResponseText, "2. *Supermercado*")
	s.Require().Len(result.State.Candidates, 2)
	s.Equal(testLeafCombustivel, result.State.Candidates[0].SubcategoryID)
}

func (s *TransactionWriteWorkflowSuite) TestResume_CategorySlot_LeafNameSelectsAndConfirms() {
	state := s.newExpenseState()
	state.Awaiting = TransactionAwaitingCategory
	state.Candidates = []PendingCategoryCandidate{
		{RootCategoryID: testRootCustoFixoID, RootSlug: "custo-fixo", SubcategoryID: testLeafCombustivel, SubcategorySlug: "combustivel", Path: "Custo Fixo > Combustível"},
		{RootCategoryID: testRootCustoFixoID, RootSlug: "custo-fixo", SubcategoryID: testLeafSupermercado, SubcategorySlug: "supermercado", Path: "Custo Fixo > Supermercado"},
	}
	k := s.key("thr-leaf-name")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("Custo fixo > combustível"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Contains(result.State.ResponseText, "Posso registrar?")
	s.Require().Len(result.State.Candidates, 1)
	s.Equal(testLeafCombustivel, result.State.Candidates[0].SubcategoryID)
}

func (s *TransactionWriteWorkflowSuite) TestResume_PaymentSlot_CardNicknameResolves() {
	cards := imocks.NewCardManager(s.T())
	cardID := uuid.New()
	cards.EXPECT().
		ResolveCardByNickname(mock.Anything, s.userID, "xp").
		Return(ifaces.Card{ID: cardID.String(), Nickname: "XP"}, nil).
		Once()
	def := BuildTransactionWriteWorkflowWithObservability(s.ledger, cards, nil, nil, nil)

	state := s.newExpenseState()
	state.Awaiting = TransactionAwaitingPaymentMethod
	state.PaymentMethod = ""
	k := s.key("thr-payment-card")

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("Cartão de crédito XP"))

	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Contains(result.State.ResponseText, "Posso registrar?")
	s.Equal(PaymentMethodCreditCard, result.State.PaymentMethod)
	s.Require().NotNil(result.State.CardID)
	s.Equal(cardID, *result.State.CardID)
}

func (s *TransactionWriteWorkflowSuite) TestResume_PaymentSlot_RestatementReplacesAndReparses() {
	state := s.newExpenseState()
	state.Awaiting = TransactionAwaitingPaymentMethod
	state.PaymentMethod = ""
	k := s.key("thr-payment-replace")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	handled, reply, err := ContinueTransactionWrite(s.ctx, s.engine, s.def, k, "Paguei 100 reais no abastecimento do veículo no cartão xp", "wamid-replace")

	s.Require().NoError(err)
	s.False(handled)
	s.Empty(reply)

	snap, found, loadErr := s.store.Load(s.ctx, TransactionWriteWorkflowID, k)
	s.Require().NoError(loadErr)
	s.Require().True(found)
	s.Equal(workflow.RunStatusSucceeded, snap.Status)
}

func (s *TransactionWriteWorkflowSuite) TestJourney_RootListing_LeafChoice_Payment_Confirm_WritesWithVersion() {
	cats := imocks.NewCategoriesReader(s.T())
	cats.EXPECT().CatalogVersion(mock.Anything).Return(int64(9), nil).Maybe()
	cats.EXPECT().
		ListCategories(mock.Anything, s.userID).
		Return([]ifaces.Category{{
			ID: testRootCustoFixoID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense",
			Subcategories: []ifaces.Category{
				{ID: testLeafCombustivel, Slug: "combustivel", Name: "Combustível"},
				{ID: testLeafSupermercado, Slug: "supermercado", Name: "Supermercado"},
			},
		}}, nil).
		Once()
	cats.EXPECT().
		SearchDictionary(mock.Anything, mock.Anything, mock.Anything).
		Return(ifaces.CategorySearchResult{Version: 9}, nil).
		Maybe()
	cats.EXPECT().
		ResolveForWrite(mock.Anything, ifaces.CategoryWriteRequest{
			RootCategoryID:  testRootCustoFixoID,
			SubcategoryID:   testLeafSupermercado,
			Kind:            ifaces.CategoryKindExpense,
			ExpectedVersion: 9,
		}).
		Return(ifaces.CategoryWriteDecision{
			RootCategoryID:   testRootCustoFixoID,
			SubcategoryID:    testLeafSupermercado,
			RootSlug:         "custo-fixo",
			SubcategorySlug:  "supermercado",
			Path:             "Custo Fixo > Supermercado",
			EditorialVersion: 9,
		}, nil).
		Once()
	txID := uuid.New()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: txID, Kind: ifaces.EntryKindTransaction}, nil).
		Once()
	def := BuildTransactionWriteWorkflowWithObservability(s.ledger, nil, cats, nil, nil)

	state := s.newRootListingState()
	state.CategoryVersion = 9
	k := s.key("thr-journey-version")

	startResult, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)
	s.Contains(startResult.State.ResponseText, "Em qual categoria isso se encaixa? 📂")

	leafList, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("1"))
	s.Require().NoError(err)
	s.Contains(leafList.State.ResponseText, "Dentro de *Custo Fixo*, qual subcategoria? 📂")

	paymentAsk, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("Supermercado"))
	s.Require().NoError(err)
	s.Contains(paymentAsk.State.ResponseText, "Como você pagou?")

	confirmAsk, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("pix"))
	s.Require().NoError(err)
	s.Contains(confirmAsk.State.ResponseText, "Posso registrar?")
	s.Contains(confirmAsk.State.ResponseText, "Custo Fixo > Supermercado")

	final, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("sim"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, final.Status)
	s.Equal(TransactionWriteStatusCompleted, final.State.Status)
	s.Equal(txID, final.State.ResourceID)
	s.Equal(int64(9), final.State.CategoryVersion)
}
