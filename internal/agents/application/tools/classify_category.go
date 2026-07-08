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
	MatchedTerm   string  `json:"matchedTerm,omitempty"`
	Score         float64 `json:"score"`
	SignalType    string  `json:"signalType,omitempty"`
	Confidence    string  `json:"confidence,omitempty"`
	MatchQuality  string  `json:"matchQuality,omitempty"`
	MatchReason   string  `json:"matchReason,omitempty"`
	IsAmbiguous   bool    `json:"isAmbiguous"`
}

type ClassifyCategoryOutput struct {
	Outcome       string                    `json:"outcome"`
	Version       int64                     `json:"version"`
	Candidates    []CategoryCandidateOutput `json:"candidates"`
	CategoryID    string                    `json:"categoryId,omitempty"`
	SubcategoryID string                    `json:"subcategoryId,omitempty"`
	Path          string                    `json:"path,omitempty"`
	IsAmbiguous   bool                      `json:"isAmbiguous"`
	WriteDecision string                    `json:"writeDecision"`
	BlockReason   string                    `json:"blockReason,omitempty"`
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
				"outcome":       map[string]any{"type": "string"},
				"version":       map[string]any{"type": "integer"},
				"candidates":    map[string]any{"type": "array"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
				"path":          map[string]any{"type": "string"},
				"isAmbiguous":   map[string]any{"type": "boolean"},
				"writeDecision": map[string]any{"type": "string"},
				"blockReason":   map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "version", "candidates", "isAmbiguous", "writeDecision"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("classify_category", "Classifica um termo em categorias financeiras para o usuário.", in, out, buildClassifyCategoryExec(reader))
}

func buildClassifyCategoryExec(reader interfaces.CategoriesReader) func(context.Context, ClassifyCategoryInput) (ClassifyCategoryOutput, error) {
	return func(ctx context.Context, in ClassifyCategoryInput) (ClassifyCategoryOutput, error) {
		result, err := reader.SearchDictionary(ctx, in.Term, in.Kind)
		if err != nil {
			return ClassifyCategoryOutput{}, fmt.Errorf("classify_category: %w", err)
		}
		mapped := make([]CategoryCandidateOutput, len(result.Candidates))
		for i, c := range result.Candidates {
			mapped[i] = CategoryCandidateOutput{
				CategoryID:    c.RootCategoryID.String(),
				SubcategoryID: c.CategoryID.String(),
				Path:          c.Path,
				MatchedTerm:   c.MatchedTerm,
				Score:         c.Score,
				SignalType:    c.SignalType,
				Confidence:    c.Confidence,
				MatchQuality:  c.MatchQuality,
				MatchReason:   c.MatchReason,
				IsAmbiguous:   c.IsAmbiguous,
			}
		}

		writeDecision, blockReason := classifyWriteDecision(result)

		output := ClassifyCategoryOutput{
			Outcome:       result.Outcome.String(),
			Version:       result.Version,
			Candidates:    mapped,
			IsAmbiguous:   writeDecision == "blocked",
			WriteDecision: writeDecision,
			BlockReason:   blockReason,
		}
		if writeDecision == "allowed" {
			output.CategoryID = result.Candidates[0].RootCategoryID.String()
			output.SubcategoryID = result.Candidates[0].CategoryID.String()
			output.Path = result.Candidates[0].Path
		}
		return output, nil
	}
}

func classifyWriteDecision(result interfaces.CategorySearchResult) (decision, reason string) {
	if result.IsWriteEligible() {
		return "allowed", ""
	}
	if result.Version <= 0 {
		return "blocked", "versão editorial ausente"
	}
	if result.Outcome != interfaces.ClassifyOutcomeMatched {
		return "blocked", "outcome não é matched: " + result.Outcome.String()
	}
	if len(result.Candidates) == 0 {
		return "blocked", "sem candidatos"
	}
	if len(result.Candidates) > 1 {
		return "blocked", "múltiplos candidatos"
	}
	c := result.Candidates[0]
	if c.IsAmbiguous {
		return "blocked", "candidato ambíguo"
	}
	if c.RootCategoryID == c.CategoryID {
		return "blocked", "raiz igual à subcategoria"
	}
	return "blocked", "não elegível para escrita"
}
