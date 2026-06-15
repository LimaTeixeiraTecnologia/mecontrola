package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

type ListCategoriesUseCase interface {
	Execute(ctx context.Context, in *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error)
}

type CategoriesAdapter struct {
	listUseCase ListCategoriesUseCase
	maxItems    int
}

func NewCategoriesAdapter(listUseCase ListCategoriesUseCase) *CategoriesAdapter {
	return &CategoriesAdapter{listUseCase: listUseCase, maxItems: 8}
}

func (a *CategoriesAdapter) List(ctx context.Context, _ json.RawMessage) (string, error) {
	result, err := a.listUseCase.Execute(ctx, &categoriesinput.ListCategoriesInput{IncludeDeprecated: false})
	if err != nil {
		return "", fmt.Errorf("categories.list: %w", err)
	}
	if result == nil || len(result.Categories) == 0 {
		return "Voce ainda nao tem categorias configuradas.", nil
	}

	names := make([]string, 0, len(result.Categories))
	for _, c := range result.Categories {
		if c.Name == "" {
			continue
		}
		names = append(names, c.Name)
	}
	total := len(names)
	limit := a.maxItems
	if limit <= 0 || limit > total {
		limit = total
	}

	preview := strings.Join(names[:limit], ", ")
	if total > limit {
		return fmt.Sprintf("Voce tem %d categorias. Algumas delas: %s.", total, preview), nil
	}
	return fmt.Sprintf("Voce tem %d categorias: %s.", total, preview), nil
}
