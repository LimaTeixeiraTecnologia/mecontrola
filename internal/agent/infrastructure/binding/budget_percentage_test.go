package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	budgetsentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type fakeEditCategoryPercentageUC struct {
	out   budgetsoutput.BudgetOutput
	err   error
	gotIn budgetsinput.EditCategoryPercentageInput
	calls int
}

func (f *fakeEditCategoryPercentageUC) Execute(_ context.Context, in budgetsinput.EditCategoryPercentageInput) (budgetsoutput.BudgetOutput, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type BudgetPercentageBindingSuite struct {
	suite.Suite
	ctx    context.Context
	userID uuid.UUID
}

func TestBudgetPercentageBindingSuite(t *testing.T) {
	suite.Run(t, new(BudgetPercentageBindingSuite))
}

func (s *BudgetPercentageBindingSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
}

func (s *BudgetPercentageBindingSuite) TestMapsCategoryNameToRootSlug() {
	uc := &fakeEditCategoryPercentageUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06"}}
	adapter := NewCategoryPercentageEditorAdapter(uc)

	result, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
		UserID:       s.userID,
		Competence:   "2026-06",
		CategoryName: "Prazeres",
		Percentage:   30,
	})
	s.Require().NoError(err)
	s.Equal(1, uc.calls)
	s.Equal("expense.prazeres", uc.gotIn.RootSlug)
	s.Equal(s.userID.String(), uc.gotIn.UserID)
	s.Equal("2026-06", uc.gotIn.Competence)
	s.Equal(30, uc.gotIn.Percentage)
	s.Equal("expense.prazeres", result.RootSlug)
	s.Equal(30, result.Percentage)
}

func (s *BudgetPercentageBindingSuite) TestMapsAllCanonicalCategoryAliases() {
	cases := []struct {
		name string
		slug string
	}{
		{"custo fixo", "expense.custo_fixo"},
		{"conhecimento", "expense.conhecimento"},
		{"educação", "expense.conhecimento"},
		{"prazeres", "expense.prazeres"},
		{"metas", "expense.metas"},
		{"liberdade financeira", "expense.liberdade_financeira"},
		{"investimentos", "expense.liberdade_financeira"},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			uc := &fakeEditCategoryPercentageUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06"}}
			adapter := NewCategoryPercentageEditorAdapter(uc)

			result, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
				UserID:       s.userID,
				Competence:   "2026-06",
				CategoryName: c.name,
				Percentage:   20,
			})
			s.Require().NoError(err)
			s.Equal(c.slug, uc.gotIn.RootSlug)
			s.Equal(c.slug, result.RootSlug)
		})
	}
}

func (s *BudgetPercentageBindingSuite) TestMapsPercentageZeroAndHundred() {
	for _, pct := range []int{0, 100} {
		s.Run("percentage", func() {
			uc := &fakeEditCategoryPercentageUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06"}}
			adapter := NewCategoryPercentageEditorAdapter(uc)

			result, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
				UserID:       s.userID,
				Competence:   "2026-06",
				CategoryName: "metas",
				Percentage:   pct,
			})
			s.Require().NoError(err)
			s.Equal(pct, uc.gotIn.Percentage)
			s.Equal(pct, result.Percentage)
		})
	}
}

func (s *BudgetPercentageBindingSuite) TestNoBudgetWhenBudgetNotActive() {
	uc := &fakeEditCategoryPercentageUC{err: budgetsentities.ErrBudgetNotActive}
	adapter := NewCategoryPercentageEditorAdapter(uc)

	_, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
		UserID:       s.userID,
		Competence:   "2026-06",
		CategoryName: "prazeres",
		Percentage:   10,
	})
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrCategoryPercentageNoBudget))
}

func (s *BudgetPercentageBindingSuite) TestUnknownCategoryReturnsAgentSentinel() {
	uc := &fakeEditCategoryPercentageUC{}
	adapter := NewCategoryPercentageEditorAdapter(uc)

	_, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
		UserID:       s.userID,
		Competence:   "2026-06",
		CategoryName: "viagens",
		Percentage:   30,
	})
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrCategoryPercentageUnknownCategory))
	s.Equal(0, uc.calls)
}

func (s *BudgetPercentageBindingSuite) TestNoBudgetReturnsAgentSentinel() {
	uc := &fakeEditCategoryPercentageUC{err: budgetsinterfaces.ErrBudgetNotFound}
	adapter := NewCategoryPercentageEditorAdapter(uc)

	_, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
		UserID:       s.userID,
		Competence:   "2026-06",
		CategoryName: "metas",
		Percentage:   25,
	})
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrCategoryPercentageNoBudget))
}

func (s *BudgetPercentageBindingSuite) TestPropagatesGenericUsecaseError() {
	uc := &fakeEditCategoryPercentageUC{err: errors.New("boom")}
	adapter := NewCategoryPercentageEditorAdapter(uc)

	_, err := adapter.Execute(s.ctx, appservices.CategoryPercentageEditorInput{
		UserID:       s.userID,
		Competence:   "2026-06",
		CategoryName: "custo fixo",
		Percentage:   40,
	})
	s.Require().Error(err)
	s.False(errors.Is(err, appservices.ErrCategoryPercentageNoBudget))
}
