package binding

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type categoriesLister interface {
	Execute(ctx context.Context, in *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error)
}

type ListCategoriesBinding struct {
	uc categoriesLister
}

func NewListCategoriesBinding(uc categoriesLister) *ListCategoriesBinding {
	return &ListCategoriesBinding{uc: uc}
}

func (b *ListCategoriesBinding) Execute(ctx context.Context, userID uuid.UUID) (tools.CategoryListResult, error) {
	out, err := b.uc.Execute(ctx, &categoriesinput.ListCategoriesInput{})
	if err != nil {
		return tools.CategoryListResult{}, fmt.Errorf("agent: list categories: %w", err)
	}
	if out == nil {
		return tools.CategoryListResult{}, nil
	}
	views := make([]tools.CategoryView, 0, len(out.Categories))
	for _, root := range out.Categories {
		views = append(views, tools.CategoryView{Slug: root.Slug, Name: root.Name})
		for _, sub := range root.Subcategories {
			views = append(views, tools.CategoryView{Slug: sub.Slug, Name: sub.Name, ParentSlug: root.Slug})
		}
	}
	return tools.CategoryListResult{Categories: views}, nil
}

type categoriesClassifier interface {
	Execute(ctx context.Context, in *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error)
}

type ClassifyCategoryBinding struct {
	uc categoriesClassifier
}

func NewClassifyCategoryBinding(uc categoriesClassifier) *ClassifyCategoryBinding {
	return &ClassifyCategoryBinding{uc: uc}
}

func (b *ClassifyCategoryBinding) Execute(ctx context.Context, in tools.CategoryClassifyInput) (tools.CategoryClassifyResult, error) {
	out, err := b.uc.Execute(ctx, &categoriesinput.SearchDictionaryInput{
		Query: in.Query,
		Kind:  categoriesvo.KindExpense,
	})
	if err != nil {
		return tools.CategoryClassifyResult{}, fmt.Errorf("agent: classify category: %w", err)
	}
	if out == nil || len(out.Candidates) == 0 {
		return tools.CategoryClassifyResult{}, nil
	}
	top := out.Candidates[0]
	return tools.CategoryClassifyResult{
		Matched: true,
		Name:    top.MatchedTerm,
		Path:    top.Path,
	}, nil
}
