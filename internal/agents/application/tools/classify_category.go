package tools

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type ClassifyCategoryInput struct {
	Term string `json:"term"`
	Kind string `json:"kind"`
}

type CategoryCandidateOutput struct {
	CategoryID    string  `json:"categoryId"`
	SubcategoryID string  `json:"subcategoryId"`
	Path          string  `json:"path"`
	Score         float64 `json:"score"`
}

type ClassifyCategoryOutput struct {
	Candidates    []CategoryCandidateOutput `json:"candidates"`
	CategoryID    string                    `json:"categoryId,omitempty"`
	SubcategoryID string                    `json:"subcategoryId,omitempty"`
	Path          string                    `json:"path,omitempty"`
	IsAmbiguous   bool                      `json:"isAmbiguous"`
}

func BuildClassifyCategoryTool(reader interfaces.CategoriesReader) tool.ToolHandle {
	in := llm.Schema{
		Name:   "classify_category_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"term": map[string]any{"type": "string"},
				"kind": map[string]any{"type": "string"},
			},
			"required":             []string{"term", "kind"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "classify_category_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"candidates":    map[string]any{"type": "array"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
				"path":          map[string]any{"type": "string"},
				"isAmbiguous":   map[string]any{"type": "boolean"},
			},
			"required":             []string{"candidates", "isAmbiguous"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[ClassifyCategoryInput, ClassifyCategoryOutput]("classify_category", "Classifica um termo em categorias financeiras para o usuário.", in, out, buildClassifyCategoryExec(reader))
}

func buildClassifyCategoryExec(reader interfaces.CategoriesReader) func(context.Context, ClassifyCategoryInput) (ClassifyCategoryOutput, error) {
	return func(ctx context.Context, in ClassifyCategoryInput) (ClassifyCategoryOutput, error) {
		candidates, err := reader.SearchDictionary(ctx, in.Term, in.Kind)
		if err != nil {
			return ClassifyCategoryOutput{}, fmt.Errorf("classify_category: %w", err)
		}
		mapped := make([]CategoryCandidateOutput, len(candidates))
		for i, c := range candidates {
			mapped[i] = CategoryCandidateOutput{
				CategoryID:    c.RootCategoryID.String(),
				SubcategoryID: c.CategoryID.String(),
				Path:          c.Path,
				Score:         c.Score,
			}
		}
		isAmbiguous := len(candidates) > 1 || (len(candidates) == 1 && candidates[0].IsAmbiguous)
		result := ClassifyCategoryOutput{
			Candidates:  mapped,
			IsAmbiguous: isAmbiguous,
		}
		if len(candidates) > 0 {
			result.CategoryID = candidates[0].RootCategoryID.String()
			result.SubcategoryID = candidates[0].CategoryID.String()
			result.Path = candidates[0].Path
		}
		return result, nil
	}
}
