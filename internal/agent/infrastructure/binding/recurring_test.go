package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type fakeRecurringTemplateCreateUC struct {
	out transactionsoutput.RecurringTemplate
	err error
}

func (f *fakeRecurringTemplateCreateUC) Execute(_ context.Context, _ transactionsinput.RawCreateRecurringTemplate) (transactionsoutput.RecurringTemplate, error) {
	return f.out, f.err
}

type fakeListRecurringTemplatesUC struct {
	out transactionsusecases.RecurringTemplatePage
	err error
}

func (f *fakeListRecurringTemplatesUC) Execute(_ context.Context, _ bool, _ string, _ int) (transactionsusecases.RecurringTemplatePage, error) {
	return f.out, f.err
}

type fakeRecurringTemplateCreatorUC struct {
	out   usecases.CreateRecurringResult
	err   error
	calls int
}

func (f *fakeRecurringTemplateCreatorUC) Execute(_ context.Context, _ usecases.CreateRecurringCommand) (usecases.CreateRecurringResult, error) {
	f.calls++
	return f.out, f.err
}

type RecurringBindingSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRecurringBindingSuite(t *testing.T) {
	suite.Run(t, new(RecurringBindingSuite))
}

func (s *RecurringBindingSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RecurringBindingSuite) TestRecurringCreator_DelegatesSuccessViaUsecase() {
	obs := fake.NewProvider()
	rootID := uuid.New()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: rootID, RootCategoryID: rootID, Path: "Despesas > Moradia", Score: 0.95},
			},
		},
	}
	creator := &fakeRecurringTemplateCreatorUC{out: usecases.CreateRecurringResult{Persisted: true}}

	uc := usecases.NewCreateRecurringFromAgent(resolver, creator, obs)
	adapter := NewRecurringCreatorAdapter(uc)

	recurringIntent, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
		AmountCents:  80000,
		Merchant:     "Aluguel",
		CategoryHint: "moradia",
		Direction:    "outcome",
		Frequency:    "monthly",
		DayOfMonth:   5,
	})
	s.Require().NoError(err)

	result, err := adapter.Execute(s.ctx, tools.RecurringCreatorInput{
		UserID: uuid.NewString(),
		Intent: recurringIntent,
	})
	s.Require().NoError(err)
	s.True(result.Persisted)
	s.Equal(1, creator.calls)
}

func (s *RecurringBindingSuite) TestRecurringCreator_TranslatesCategoryNotFoundError() {
	obs := fake.NewProvider()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "Despesas > Alimentação", Score: 0.1},
			},
		},
	}
	creator := &fakeRecurringTemplateCreatorUC{}

	uc := usecases.NewCreateRecurringFromAgent(resolver, creator, obs)
	adapter := NewRecurringCreatorAdapter(uc)

	recurringIntent, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
		AmountCents:  80000,
		CategoryHint: "moradia",
		Direction:    "outcome",
		Frequency:    "monthly",
		DayOfMonth:   5,
	})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, tools.RecurringCreatorInput{
		UserID: uuid.NewString(),
		Intent: recurringIntent,
	})
	s.Require().Error(err)
	s.True(errors.Is(err, tools.ErrCategoryNotFound))
}

func (s *RecurringBindingSuite) TestRecurringCreator_PropagatesCreatorError() {
	obs := fake.NewProvider()
	rootID := uuid.New()
	resolver := &fakeCategoryResolver{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: rootID, RootCategoryID: rootID, Path: "Despesas > Moradia", Score: 0.95},
			},
		},
	}
	creator := &fakeRecurringTemplateCreatorUC{err: errors.New("persistencia falhou")}

	uc := usecases.NewCreateRecurringFromAgent(resolver, creator, obs)
	adapter := NewRecurringCreatorAdapter(uc)

	recurringIntent, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
		AmountCents:  80000,
		CategoryHint: "moradia",
		Direction:    "outcome",
		Frequency:    "monthly",
		DayOfMonth:   5,
	})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, tools.RecurringCreatorInput{
		UserID: uuid.NewString(),
		Intent: recurringIntent,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "persistencia falhou")
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_InvalidUserIDReturnsError() {
	adapter := NewRecurringTemplateCreatorAdapter(&fakeRecurringTemplateCreateUC{})
	_, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         "not-a-uuid",
		RootCategoryID: uuid.NewString(),
		Direction:      "outcome",
		Frequency:      "monthly",
		DayOfMonth:     5,
		AmountCents:    10000,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_InvalidRootCategoryIDReturnsError() {
	adapter := NewRecurringTemplateCreatorAdapter(&fakeRecurringTemplateCreateUC{})
	_, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: "not-a-uuid",
		Direction:      "outcome",
		Frequency:      "monthly",
		DayOfMonth:     5,
		AmountCents:    10000,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "category id")
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_InvalidSubcategoryIDReturnsError() {
	adapter := NewRecurringTemplateCreatorAdapter(&fakeRecurringTemplateCreateUC{})
	_, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  "bad-uuid",
		Direction:      "outcome",
		Frequency:      "monthly",
		DayOfMonth:     5,
		AmountCents:    10000,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "subcategory id")
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_SuccessDelegates() {
	ucFake := &fakeRecurringTemplateCreateUC{
		out: transactionsoutput.RecurringTemplate{AmountCents: 10000, Direction: "outcome"},
	}
	adapter := NewRecurringTemplateCreatorAdapter(ucFake)

	result, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		Direction:      "outcome",
		Frequency:      "monthly",
		DayOfMonth:     10,
		AmountCents:    10000,
	})
	s.Require().NoError(err)
	s.True(result.Persisted)
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_PropagatesUsecaseError() {
	ucFake := &fakeRecurringTemplateCreateUC{err: errors.New("db down")}
	adapter := NewRecurringTemplateCreatorAdapter(ucFake)

	_, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		Direction:      "outcome",
		Frequency:      "monthly",
		DayOfMonth:     10,
		AmountCents:    10000,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "db down")
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_IncomeDirectionUsesPaymentMethodTed() {
	ucFake := &fakeRecurringTemplateCreateUC{}
	adapter := NewRecurringTemplateCreatorAdapter(ucFake)

	_, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		Direction:      "income",
		Frequency:      "monthly",
		DayOfMonth:     5,
		AmountCents:    50000,
	})
	s.Require().NoError(err)
}

func (s *RecurringBindingSuite) TestRecurringTemplateCreator_EmptySubcategoryIsAllowed() {
	ucFake := &fakeRecurringTemplateCreateUC{}
	adapter := NewRecurringTemplateCreatorAdapter(ucFake)

	_, err := adapter.Execute(s.ctx, usecases.CreateRecurringCommand{
		UserID:         uuid.NewString(),
		RootCategoryID: uuid.NewString(),
		SubcategoryID:  "   ",
		Direction:      "outcome",
		Frequency:      "monthly",
		DayOfMonth:     5,
		AmountCents:    10000,
	})
	s.Require().NoError(err)
}

func (s *RecurringBindingSuite) TestRecurringLister_InvalidUserIDReturnsError() {
	adapter := NewRecurringListerAdapter(&fakeListRecurringTemplatesUC{})
	_, err := adapter.Execute(s.ctx, "not-a-uuid")
	s.Require().Error(err)
	s.Contains(err.Error(), "user id")
}

func (s *RecurringBindingSuite) TestRecurringLister_SuccessMapsTemplates() {
	ucFake := &fakeListRecurringTemplatesUC{
		out: transactionsusecases.RecurringTemplatePage{
			Templates: []transactionsoutput.RecurringTemplate{
				{AmountCents: 80000, Direction: "outcome", Description: "Aluguel", Frequency: "monthly", DayOfMonth: 5},
				{AmountCents: 3000, Direction: "income", Description: "Salário", Frequency: "monthly", DayOfMonth: 1},
			},
		},
	}
	adapter := NewRecurringListerAdapter(ucFake)

	views, err := adapter.Execute(s.ctx, uuid.NewString())
	s.Require().NoError(err)
	s.Len(views, 2)
	s.Equal("Aluguel", views[0].Description)
	s.Equal(int64(80000), views[0].AmountCents)
	s.Equal("Salário", views[1].Description)
}

func (s *RecurringBindingSuite) TestRecurringLister_EmptyListReturnsEmptySlice() {
	ucFake := &fakeListRecurringTemplatesUC{
		out: transactionsusecases.RecurringTemplatePage{Templates: nil},
	}
	adapter := NewRecurringListerAdapter(ucFake)

	views, err := adapter.Execute(s.ctx, uuid.NewString())
	s.Require().NoError(err)
	s.Empty(views)
}

func (s *RecurringBindingSuite) TestRecurringLister_PropagatesUsecaseError() {
	ucFake := &fakeListRecurringTemplatesUC{err: errors.New("db down")}
	adapter := NewRecurringListerAdapter(ucFake)

	_, err := adapter.Execute(s.ctx, uuid.NewString())
	s.Require().Error(err)
	s.Contains(err.Error(), "db down")
}
