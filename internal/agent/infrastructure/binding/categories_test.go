package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type fakeCategoriesListerUC struct {
	out   *categoriesoutput.ListCategoriesOutput
	err   error
	gotIn *categoriesinput.ListCategoriesInput
	calls int
}

func (f *fakeCategoriesListerUC) Execute(_ context.Context, in *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type fakeCategoriesClassifierUC struct {
	out   *categoriesoutput.DictionarySearchOutput
	err   error
	gotIn *categoriesinput.SearchDictionaryInput
	calls int
}

func (f *fakeCategoriesClassifierUC) Execute(_ context.Context, in *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type CategoriesBindingSuite struct {
	suite.Suite
}

func TestCategoriesBindingSuite(t *testing.T) {
	suite.Run(t, new(CategoriesBindingSuite))
}

func (s *CategoriesBindingSuite) TestListCategories_FlattensTree() {
	uc := &fakeCategoriesListerUC{out: &categoriesoutput.ListCategoriesOutput{
		Categories: []categoriesoutput.CategoryTreeOutput{
			{
				Slug: "alimentacao",
				Name: "Alimentação",
				Subcategories: []categoriesoutput.CategoryOutput{
					{Slug: "restaurante", Name: "Restaurante"},
					{Slug: "mercado", Name: "Mercado"},
				},
			},
			{Slug: "transporte", Name: "Transporte"},
		},
	}}
	binding := NewListCategoriesBinding(uc)

	result, err := binding.Execute(context.Background(), uuid.New())
	s.Require().NoError(err)
	s.Equal(1, uc.calls)
	s.Require().Len(result.Categories, 4)
	s.Equal(tools.CategoryView{Slug: "alimentacao", Name: "Alimentação"}, result.Categories[0])
	s.Equal(tools.CategoryView{Slug: "restaurante", Name: "Restaurante", ParentSlug: "alimentacao"}, result.Categories[1])
	s.Equal(tools.CategoryView{Slug: "mercado", Name: "Mercado", ParentSlug: "alimentacao"}, result.Categories[2])
	s.Equal(tools.CategoryView{Slug: "transporte", Name: "Transporte"}, result.Categories[3])
}

func (s *CategoriesBindingSuite) TestListCategories_PropagatesError() {
	uc := &fakeCategoriesListerUC{err: errors.New("boom")}
	binding := NewListCategoriesBinding(uc)

	_, err := binding.Execute(context.Background(), uuid.New())
	s.Require().Error(err)
	s.Contains(err.Error(), "agent: list categories")
}

func (s *CategoriesBindingSuite) TestClassify_MapsQueryAndExpenseKind() {
	uc := &fakeCategoriesClassifierUC{out: &categoriesoutput.DictionarySearchOutput{
		Result: "candidates",
		Candidates: []categoriesoutput.CandidateOutput{
			{Path: "Alimentação > Delivery", MatchedTerm: "ifood"},
		},
	}}
	binding := NewClassifyCategoryBinding(uc)

	result, err := binding.Execute(context.Background(), tools.CategoryClassifyInput{Query: "ifood"})
	s.Require().NoError(err)
	s.Equal(1, uc.calls)
	s.Equal("ifood", uc.gotIn.Query)
	s.Equal(categoriesvo.KindExpense, uc.gotIn.Kind)
	s.True(result.Matched)
	s.Equal("ifood", result.Name)
	s.Equal("Alimentação > Delivery", result.Path)
}

func (s *CategoriesBindingSuite) TestClassify_NoMatch() {
	uc := &fakeCategoriesClassifierUC{out: &categoriesoutput.DictionarySearchOutput{
		Result:     "no_match",
		Candidates: nil,
	}}
	binding := NewClassifyCategoryBinding(uc)

	result, err := binding.Execute(context.Background(), tools.CategoryClassifyInput{Query: "xpto"})
	s.Require().NoError(err)
	s.False(result.Matched)
	s.Empty(result.Path)
}

func (s *CategoriesBindingSuite) TestClassify_PropagatesError() {
	uc := &fakeCategoriesClassifierUC{err: errors.New("boom")}
	binding := NewClassifyCategoryBinding(uc)

	_, err := binding.Execute(context.Background(), tools.CategoryClassifyInput{Query: "ifood"})
	s.Require().Error(err)
	s.Contains(err.Error(), "agent: classify category")
}
