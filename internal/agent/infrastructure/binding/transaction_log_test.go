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
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type fakeTransactionCreateUC struct {
	out transactionsoutput.Transaction
	err error
}

func (f *fakeTransactionCreateUC) Execute(_ context.Context, _ transactionsinput.RawCreateTransaction) (transactionsoutput.Transaction, error) {
	return f.out, f.err
}

type fakeCategoryResolver struct {
	out *categoriesoutput.DictionarySearchOutput
	err error
}

func (f *fakeCategoryResolver) Execute(_ context.Context, _ *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error) {
	return f.out, f.err
}

type fakeTransactionCreatorUC struct {
	out   usecases.CreateTransactionResult
	err   error
	calls int
}

func (f *fakeTransactionCreatorUC) Execute(_ context.Context, _ usecases.CreateTransactionCommand) (usecases.CreateTransactionResult, error) {
	f.calls++
	return f.out, f.err
}

type TransactionLogBindingSuite struct {
	suite.Suite
	ctx context.Context
}

func TestTransactionLogBindingSuite(t *testing.T) {
	suite.Run(t, new(TransactionLogBindingSuite))
}

func (s *TransactionLogBindingSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_InvalidUserIDReturnsError() {
	adapter := NewTransactionCreatorAdapter(&fakeTransactionCreateUC{})
	_, err := adapter.Execute(s.ctx, usecases.CreateTransactionCommand{
		UserID:         "not-a-uuid",
		RootCategoryID: uuid.NewString(),
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_InvalidRootCategoryIDReturnsError() {
	adapter := NewTransactionCreatorAdapter(&fakeTransactionCreateUC{})
	_, err := adapter.Execute(s.ctx, usecases.CreateTransactionCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: "not-a-uuid",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "category id")
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_InvalidSubcategoryIDReturnsError() {
	adapter := NewTransactionCreatorAdapter(&fakeTransactionCreateUC{})
	_, err := adapter.Execute(s.ctx, usecases.CreateTransactionCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  "bad-uuid",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "subcategory id")
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_SuccessInjectsPrincipalAndDelegates() {
	ucFake := &fakeTransactionCreateUC{
		out: transactionsoutput.Transaction{AmountCents: 5000, Direction: "outcome"},
	}
	adapter := NewTransactionCreatorAdapter(ucFake)

	userID := uuid.NewString()
	categoryID := uuid.NewString()
	now := time.Now().UTC()

	result, err := adapter.Execute(s.ctx, usecases.CreateTransactionCommand{
		UserID:         userID,
		RootCategoryID: categoryID,
		Direction:      "outcome",
		PaymentMethod:  "pix",
		AmountCents:    5000,
		OccurredAt:     now,
	})
	s.Require().NoError(err)
	s.Equal(int64(5000), result.AmountCents)
	s.Equal("outcome", result.Direction)
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_SkipsInjectingPrincipalWhenAlreadyPresent() {
	ucFake := &fakeTransactionCreateUC{
		out: transactionsoutput.Transaction{AmountCents: 1000, Direction: "income"},
	}
	adapter := NewTransactionCreatorAdapter(ucFake)

	existing := auth.Principal{UserID: uuid.New(), Source: auth.SourceWhatsApp}
	ctx := auth.WithPrincipal(s.ctx, existing)

	result, err := adapter.Execute(ctx, usecases.CreateTransactionCommand{
		UserID:         existing.UserID.String(),
		RootCategoryID: uuid.NewString(),
		Direction:      "income",
		AmountCents:    1000,
		OccurredAt:     time.Now().UTC(),
	})
	s.Require().NoError(err)
	s.Equal(int64(1000), result.AmountCents)
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_PropagatesUsecaseError() {
	ucFake := &fakeTransactionCreateUC{err: errors.New("db down")}
	adapter := NewTransactionCreatorAdapter(ucFake)

	_, err := adapter.Execute(s.ctx, usecases.CreateTransactionCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		Direction:      "outcome",
		AmountCents:    100,
		OccurredAt:     time.Now().UTC(),
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "db down")
}

func (s *TransactionLogBindingSuite) TestTransactionCreator_EmptySubcategoryIsAllowed() {
	ucFake := &fakeTransactionCreateUC{
		out: transactionsoutput.Transaction{AmountCents: 200, Direction: "outcome"},
	}
	adapter := NewTransactionCreatorAdapter(ucFake)

	result, err := adapter.Execute(s.ctx, usecases.CreateTransactionCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  "   ",
		Direction:      "outcome",
		AmountCents:    200,
		OccurredAt:     time.Now().UTC(),
	})
	s.Require().NoError(err)
	s.Equal(int64(200), result.AmountCents)
}

func (s *TransactionLogBindingSuite) TestTransactionLogger_DelegatesSuccessViaForceCategory() {
	obs := fake.NewProvider()
	resolver := &fakeCategoryResolver{}
	creator := &fakeTransactionCreatorUC{
		out: usecases.CreateTransactionResult{AmountCents: 3000, Direction: "outcome"},
	}

	uc := usecases.NewRecordTransactionFromAgent(resolver, creator, obs)
	adapter := NewTransactionLoggerAdapter(uc)

	forced := "Despesas > Alimentação"
	in := tools.ExpenseRecorderInput{
		UserID:        uuid.NewString(),
		ForceCategory: &forced,
		AmountCents:   3000,
		Direction:     "outcome",
		PaymentMethod: "pix",
		Merchant:      "Supermercado",
	}

	result, err := adapter.Execute(s.ctx, in)
	s.Require().NoError(err)
	s.True(result.Persisted)
	s.Equal(int64(3000), result.AmountCents)
	s.Equal(1, creator.calls)
}

func (s *TransactionLogBindingSuite) TestTransactionLogger_PropagatesCreatorError() {
	obs := fake.NewProvider()
	resolver := &fakeCategoryResolver{}
	creator := &fakeTransactionCreatorUC{err: errors.New("persistencia falhou")}

	uc := usecases.NewRecordTransactionFromAgent(resolver, creator, obs)
	adapter := NewTransactionLoggerAdapter(uc)

	forced := "Despesas > Alimentação"
	in := tools.ExpenseRecorderInput{
		UserID:        uuid.NewString(),
		ForceCategory: &forced,
		AmountCents:   3000,
		Direction:     "outcome",
		PaymentMethod: "pix",
	}

	_, err := adapter.Execute(s.ctx, in)
	s.Require().Error(err)
	s.Contains(err.Error(), "persistencia falhou")
}

func (s *TransactionLogBindingSuite) TestTransactionLogger_TranslatesCategoryNotFoundError() {
	obs := fake.NewProvider()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "Despesas > Alimentação", Score: 0.1},
			},
		},
	}
	creator := &fakeTransactionCreatorUC{}

	uc := usecases.NewRecordTransactionFromAgent(resolver, creator, obs)
	adapter := NewTransactionLoggerAdapter(uc)

	expenseIntent, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  1000,
		CategoryHint: "supermercado",
	})
	s.Require().NoError(err)

	in := tools.ExpenseRecorderInput{
		UserID:        uuid.NewString(),
		Intent:        expenseIntent,
		AmountCents:   1000,
		Direction:     "outcome",
		PaymentMethod: "pix",
	}

	_, err = adapter.Execute(s.ctx, in)
	s.Require().Error(err)
	s.True(errors.Is(err, tools.ErrCategoryNotFound))
}

func (s *TransactionLogBindingSuite) TestTransactionLogger_TranslatesCategoryAmbiguousError() {
	obs := fake.NewProvider()
	cat1 := uuid.New()
	cat2 := uuid.New()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: cat1, RootCategoryID: cat1, Path: "Despesas > Lazer", Score: 0.9, IsAmbiguous: true},
				{CategoryID: cat2, RootCategoryID: cat2, Path: "Despesas > Prazeres", Score: 0.85},
			},
		},
	}
	creator := &fakeTransactionCreatorUC{}

	uc := usecases.NewRecordTransactionFromAgent(resolver, creator, obs)
	adapter := NewTransactionLoggerAdapter(uc)

	expenseIntent, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  1000,
		CategoryHint: "academia",
	})
	s.Require().NoError(err)

	in := tools.ExpenseRecorderInput{
		UserID:        uuid.NewString(),
		Intent:        expenseIntent,
		AmountCents:   1000,
		Direction:     "outcome",
		PaymentMethod: "pix",
	}

	_, err = adapter.Execute(s.ctx, in)
	s.Require().Error(err)
	var ambiguous *tools.CategoryAmbiguousError
	s.True(errors.As(err, &ambiguous))
}
