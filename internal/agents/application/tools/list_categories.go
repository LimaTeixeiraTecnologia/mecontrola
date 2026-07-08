package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type ListCategoriesInput struct{}

type ListCategoriesSubcategoryOutput struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	AllocationType string `json:"allocationType"`
}

type ListCategoriesItemOutput struct {
	ID             string                            `json:"id"`
	Slug           string                            `json:"slug"`
	Name           string                            `json:"name"`
	Kind           string                            `json:"kind"`
	AllocationType string                            `json:"allocationType"`
	Subcategories  []ListCategoriesSubcategoryOutput `json:"subcategories"`
}

type ListCategoriesOutput struct {
	Categories []ListCategoriesItemOutput `json:"categories"`
}

func BuildListCategoriesTool(reader interfaces.CategoriesReader) tool.ToolHandle {
	in := llm.Schema{
		Name:   "list_categories_input",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "list_categories_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"categories": map[string]any{"type": "array"},
			},
			"required":             []string{"categories"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("list_categories", "Lista as categorias e subcategorias disponíveis para o usuário.", in, out, buildListCategoriesExec(reader))
}

func buildListCategoriesExec(reader interfaces.CategoriesReader) func(context.Context, ListCategoriesInput) (ListCategoriesOutput, error) {
	return func(ctx context.Context, _ ListCategoriesInput) (ListCategoriesOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return ListCategoriesOutput{}, fmt.Errorf("list_categories: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return ListCategoriesOutput{}, fmt.Errorf("list_categories: userId inválido: %w", err)
		}
		cats, err := reader.ListCategories(ctx, userID)
		if err != nil {
			return ListCategoriesOutput{}, fmt.Errorf("list_categories: %w", err)
		}
		out := make([]ListCategoriesItemOutput, len(cats))
		for i, c := range cats {
			subs := make([]ListCategoriesSubcategoryOutput, len(c.Subcategories))
			for j, s := range c.Subcategories {
				subs[j] = ListCategoriesSubcategoryOutput{
					ID:             s.ID.String(),
					Slug:           s.Slug,
					Name:           s.Name,
					Kind:           s.Kind,
					AllocationType: s.AllocationType,
				}
			}
			out[i] = ListCategoriesItemOutput{
				ID:             c.ID.String(),
				Slug:           c.Slug,
				Name:           c.Name,
				Kind:           c.Kind,
				AllocationType: c.AllocationType,
				Subcategories:  subs,
			}
		}
		return ListCategoriesOutput{Categories: out}, nil
	}
}
