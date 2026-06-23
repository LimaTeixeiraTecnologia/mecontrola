package binding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type fakeListTransactionsUC struct {
	out transactionsusecases.TransactionPage
	err error
}

func (f *fakeListTransactionsUC) Execute(_ context.Context, _, _ string, _ int) (transactionsusecases.TransactionPage, error) {
	return f.out, f.err
}

type fakeDeleteTransactionUC struct {
	err   error
	calls int
}

func (f *fakeDeleteTransactionUC) Execute(_ context.Context, _ string, _ int64) error {
	f.calls++
	return f.err
}

type fakeGetTransactionUC struct {
	out transactionsoutput.Transaction
	err error
}

func (f *fakeGetTransactionUC) Execute(_ context.Context, _ string) (transactionsoutput.Transaction, error) {
	return f.out, f.err
}

type fakeUpdateTransactionUC struct {
	out   transactionsoutput.Transaction
	err   error
	calls int
}

func (f *fakeUpdateTransactionUC) Execute(_ context.Context, _ string, _ transactionsinput.RawUpdateTransaction) (transactionsoutput.Transaction, error) {
	f.calls++
	return f.out, f.err
}

type fakeCardPurchaseLoggerCreatorUC struct {
	out usecases.CreateCardPurchaseResult
	err error
}

func (f *fakeCardPurchaseLoggerCreatorUC) Execute(_ context.Context, _ usecases.CreateCardPurchaseCommand) (usecases.CreateCardPurchaseResult, error) {
	return f.out, f.err
}

type fakeCardPurchaseCreateUC struct {
	out transactionsoutput.CardPurchase
	err error
}

func (f *fakeCardPurchaseCreateUC) Execute(_ context.Context, _ transactionsinput.RawCreateCardPurchase) (transactionsoutput.CardPurchase, error) {
	return f.out, f.err
}

type TransactionQuerySuite struct {
	suite.Suite
	ctx    context.Context
	userID uuid.UUID
	cardID string
}

func TestTransactionQuerySuite(t *testing.T) {
	suite.Run(t, new(TransactionQuerySuite))
}

func (s *TransactionQuerySuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
	s.cardID = uuid.NewString()
}

func (s *TransactionQuerySuite) sampleTransaction() transactionsoutput.Transaction {
	return transactionsoutput.Transaction{
		ID:          uuid.New(),
		UserID:      s.userID,
		Direction:   "outcome",
		AmountCents: 5000,
		Description: "Supermercado",
		OccurredAt:  time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		Version:     1,
	}
}

func (s *TransactionQuerySuite) cardList() cardoutput.CardList {
	return cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Nubank Roxinho", Nickname: "nubank"},
	}}
}

func (s *TransactionQuerySuite) TestTransactionLister_Success() {
	tx := s.sampleTransaction()
	uc := &fakeListTransactionsUC{
		out: transactionsusecases.TransactionPage{
			Transactions: []transactionsoutput.Transaction{tx},
		},
	}
	adapter := NewTransactionListerAdapter(uc)

	result, err := adapter.Execute(s.ctx, tools.TransactionListInput{
		UserID:   s.userID.String(),
		RefMonth: "2026-06",
	})

	s.Require().NoError(err)
	s.Equal("2026-06", result.RefMonth)
	s.Len(result.Transactions, 1)
	s.Equal(tx.ID.String(), result.Transactions[0].ID)
	s.Equal(int64(5000), result.Transactions[0].AmountCents)
}

func (s *TransactionQuerySuite) TestTransactionLister_InvalidUserIDReturnsError() {
	uc := &fakeListTransactionsUC{}
	adapter := NewTransactionListerAdapter(uc)

	_, err := adapter.Execute(s.ctx, tools.TransactionListInput{
		UserID:   "not-a-uuid",
		RefMonth: "2026-06",
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
}

func (s *TransactionQuerySuite) TestTransactionLister_PropagatesUsecaseError() {
	uc := &fakeListTransactionsUC{err: errors.New("db down")}
	adapter := NewTransactionListerAdapter(uc)

	_, err := adapter.Execute(s.ctx, tools.TransactionListInput{
		UserID:   s.userID.String(),
		RefMonth: "2026-06",
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "db down")
}

func (s *TransactionQuerySuite) TestTransactionLister_EmptyListReturnsSuccess() {
	uc := &fakeListTransactionsUC{
		out: transactionsusecases.TransactionPage{},
	}
	adapter := NewTransactionListerAdapter(uc)

	result, err := adapter.Execute(s.ctx, tools.TransactionListInput{
		UserID:   s.userID.String(),
		RefMonth: "2026-06",
	})

	s.Require().NoError(err)
	s.Empty(result.Transactions)
}

func (s *TransactionQuerySuite) TestLastTransactionDeleter_Success() {
	uc := &fakeDeleteTransactionUC{}
	adapter := NewLastTransactionDeleterAdapter(uc)

	txID := uuid.NewString()
	err := adapter.Execute(s.ctx, s.userID.String(), txID, 1)

	s.Require().NoError(err)
	s.Equal(1, uc.calls)
}

func (s *TransactionQuerySuite) TestLastTransactionDeleter_InvalidUserIDReturnsError() {
	uc := &fakeDeleteTransactionUC{}
	adapter := NewLastTransactionDeleterAdapter(uc)

	err := adapter.Execute(s.ctx, "bad-uuid", uuid.NewString(), 1)

	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
	s.Equal(0, uc.calls)
}

func (s *TransactionQuerySuite) TestLastTransactionDeleter_PropagatesUsecaseError() {
	uc := &fakeDeleteTransactionUC{err: errors.New("delete failed")}
	adapter := NewLastTransactionDeleterAdapter(uc)

	err := adapter.Execute(s.ctx, s.userID.String(), uuid.NewString(), 1)

	s.Require().Error(err)
	s.Contains(err.Error(), "delete failed")
	s.Equal(1, uc.calls)
}

func (s *TransactionQuerySuite) TestLastTransactionEditor_Success() {
	tx := s.sampleTransaction()
	getUC := &fakeGetTransactionUC{out: tx}
	updatedTx := tx
	updatedTx.AmountCents = 8000
	updateUC := &fakeUpdateTransactionUC{out: updatedTx}
	adapter := NewLastTransactionEditorAdapter(getUC, updateUC)

	result, err := adapter.Execute(s.ctx, tools.EditTransactionInput{
		UserID: s.userID.String(),
		Current: tools.TransactionView{
			ID:      tx.ID.String(),
			Version: 1,
		},
		NewAmount: 8000,
	})

	s.Require().NoError(err)
	s.True(result.Persisted)
	s.Equal(int64(5000), result.OldAmount)
	s.Equal(int64(8000), result.NewAmount)
	s.Equal(1, updateUC.calls)
}

func (s *TransactionQuerySuite) TestLastTransactionEditor_InvalidUserIDReturnsError() {
	getUC := &fakeGetTransactionUC{}
	updateUC := &fakeUpdateTransactionUC{}
	adapter := NewLastTransactionEditorAdapter(getUC, updateUC)

	_, err := adapter.Execute(s.ctx, tools.EditTransactionInput{
		UserID: "not-a-uuid",
		Current: tools.TransactionView{
			ID: uuid.NewString(),
		},
		NewAmount: 1000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
	s.Equal(0, updateUC.calls)
}

func (s *TransactionQuerySuite) TestLastTransactionEditor_GetUsecaseErrorIsWrapped() {
	getUC := &fakeGetTransactionUC{err: errors.New("not found")}
	updateUC := &fakeUpdateTransactionUC{}
	adapter := NewLastTransactionEditorAdapter(getUC, updateUC)

	_, err := adapter.Execute(s.ctx, tools.EditTransactionInput{
		UserID: s.userID.String(),
		Current: tools.TransactionView{
			ID: uuid.NewString(),
		},
		NewAmount: 1000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "buscar atual")
	s.Equal(0, updateUC.calls)
}

func (s *TransactionQuerySuite) TestLastTransactionEditor_UpdateUsecaseErrorIsWrapped() {
	tx := s.sampleTransaction()
	getUC := &fakeGetTransactionUC{out: tx}
	updateUC := &fakeUpdateTransactionUC{err: errors.New("update failed")}
	adapter := NewLastTransactionEditorAdapter(getUC, updateUC)

	_, err := adapter.Execute(s.ctx, tools.EditTransactionInput{
		UserID: s.userID.String(),
		Current: tools.TransactionView{
			ID:      tx.ID.String(),
			Version: 1,
		},
		NewAmount: 9000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "atualizar")
	s.Equal(1, updateUC.calls)
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_Success() {
	lister := &fakeListCardsUC{out: s.cardList()}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	result, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		CardHint:       "nubank",
		RootCategoryID: uuid.NewString(),
		AmountCents:    10000,
		Installments:   3,
		Description:    "TV",
	})

	s.Require().NoError(err)
	s.True(result.CardFound)
	s.Equal("nubank", result.CardName)
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_InvalidUserIDReturnsError() {
	lister := &fakeListCardsUC{}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	_, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         "bad-uuid",
		RootCategoryID: uuid.NewString(),
		CardHint:       "nubank",
		AmountCents:    1000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_InvalidCategoryIDReturnsError() {
	lister := &fakeListCardsUC{}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	_, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: "not-a-uuid",
		CardHint:       "nubank",
		AmountCents:    1000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "category id")
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_InvalidSubcategoryIDReturnsError() {
	lister := &fakeListCardsUC{}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	_, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  "invalid",
		CardHint:       "nubank",
		AmountCents:    1000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "subcategory id")
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_CardNotFoundReturnsFalse() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{}}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	result, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		CardHint:       "inexistente",
		AmountCents:    1000,
	})

	s.Require().NoError(err)
	s.False(result.CardFound)
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_ListerErrorIsWrapped() {
	lister := &fakeListCardsUC{err: errors.New("db down")}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	_, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		CardHint:       "nubank",
		AmountCents:    1000,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "listar cartões")
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_CreateUsecaseErrorIsWrapped() {
	lister := &fakeListCardsUC{out: s.cardList()}
	createUC := &fakeCardPurchaseCreateUC{err: errors.New("create boom")}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	_, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		CardHint:       "nubank",
		AmountCents:    5000,
		Installments:   1,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "criar")
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_EmptySubcategoryIsAllowed() {
	lister := &fakeListCardsUC{out: s.cardList()}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	result, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  "   ",
		CardHint:       "nubank",
		AmountCents:    3000,
		Installments:   1,
	})

	s.Require().NoError(err)
	s.True(result.CardFound)
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_FallsBackToNameWhenNicknameEmpty() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Nubank Roxinho", Nickname: "   "},
	}}}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	result, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		CardHint:       "Nubank Roxinho",
		AmountCents:    2000,
		Installments:   2,
	})

	s.Require().NoError(err)
	s.True(result.CardFound)
	s.Equal("Nubank Roxinho", result.CardName)
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_ResolvesByPartialContains() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Cartao Itau Black", Nickname: "itau black"},
	}}}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	result, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		CardHint:       "itau",
		AmountCents:    1500,
		Installments:   1,
	})

	s.Require().NoError(err)
	s.True(result.CardFound)
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_InvalidCardIDInListIsWrapped() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: "not-a-uuid", Name: "Nubank Roxinho", Nickname: "nubank"},
	}}}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	_, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		CardHint:       "nubank",
		AmountCents:    1000,
		Installments:   1,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "card id")
}

func (s *TransactionQuerySuite) TestCardPurchaseCreator_WithValidSubcategory() {
	lister := &fakeListCardsUC{out: s.cardList()}
	createUC := &fakeCardPurchaseCreateUC{}
	adapter := NewCardPurchaseCreatorAdapter(lister, createUC)

	result, err := adapter.Execute(s.ctx, usecases.CreateCardPurchaseCommand{
		UserID:         s.userID.String(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  uuid.NewString(),
		CardHint:       "nubank",
		AmountCents:    7500,
		Installments:   5,
	})

	s.Require().NoError(err)
	s.True(result.CardFound)
}

func (s *TransactionQuerySuite) buildCardPurchaseIntent(merchant, categoryHint string, amountCents int64) intent.Intent {
	i, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{
		AmountCents:  amountCents,
		Merchant:     merchant,
		CategoryHint: categoryHint,
		Installments: 2,
	})
	s.Require().NoError(err)
	return i
}

func (s *TransactionQuerySuite) TestCardPurchaseLogger_TranslatesCategoryNotFoundError() {
	obs := fake.NewProvider()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "Lazer > TV e Streaming", Score: 0.1},
			},
		},
	}
	creator := &fakeCardPurchaseLoggerCreatorUC{}

	uc := usecases.NewRecordCardPurchaseFromAgent(resolver, creator, obs)
	adapter := NewCardPurchaseLoggerAdapter(uc)

	in := tools.CardPurchaseLoggerInput{
		UserID: s.userID.String(),
		Intent: s.buildCardPurchaseIntent("TV", "", 180000),
	}

	_, err := adapter.Execute(s.ctx, in)
	s.Require().Error(err)
	s.True(errors.Is(err, tools.ErrCategoryNotFound))
}

func (s *TransactionQuerySuite) TestCardPurchaseLogger_TranslatesCategoryAmbiguousError() {
	obs := fake.NewProvider()
	cat1 := uuid.New()
	cat2 := uuid.New()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: cat1, RootCategoryID: cat1, Path: "Lazer > Streaming", Score: 0.9, IsAmbiguous: true},
				{CategoryID: cat2, RootCategoryID: cat2, Path: "Lazer > TV", Score: 0.85},
			},
		},
	}
	creator := &fakeCardPurchaseLoggerCreatorUC{}

	uc := usecases.NewRecordCardPurchaseFromAgent(resolver, creator, obs)
	adapter := NewCardPurchaseLoggerAdapter(uc)

	in := tools.CardPurchaseLoggerInput{
		UserID: s.userID.String(),
		Intent: s.buildCardPurchaseIntent("Netflix", "streaming", 180000),
	}

	_, err := adapter.Execute(s.ctx, in)
	s.Require().Error(err)
	var ambiguous *tools.CategoryAmbiguousError
	s.True(errors.As(err, &ambiguous))
}

func (s *TransactionQuerySuite) TestCardPurchaseLogger_TranslatesCategoryNeedsConfirmationError() {
	obs := fake.NewProvider()
	cat1 := uuid.New()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: cat1, RootCategoryID: cat1, Path: "Lazer > TV e Streaming", Score: 0.75},
			},
		},
	}
	creator := &fakeCardPurchaseLoggerCreatorUC{}

	uc := usecases.NewRecordCardPurchaseFromAgent(resolver, creator, obs)
	adapter := NewCardPurchaseLoggerAdapter(uc)

	in := tools.CardPurchaseLoggerInput{
		UserID: s.userID.String(),
		Intent: s.buildCardPurchaseIntent("TV", "", 180000),
	}

	_, err := adapter.Execute(s.ctx, in)
	s.Require().Error(err)
	var needsConfirmation *tools.CategoryNeedsConfirmationError
	s.True(errors.As(err, &needsConfirmation))
}
