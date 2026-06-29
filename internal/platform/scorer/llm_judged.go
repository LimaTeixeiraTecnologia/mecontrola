package scorer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type judgeResponse struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

func (r judgeResponse) validate() error {
	if r.Score < 0 || r.Score > 1 {
		return fmt.Errorf("%w: got %.3f", ErrInvalidScore, r.Score)
	}
	return nil
}

type judgeContract struct{}

func (judgeContract) Schema() llm.Schema {
	return llm.Schema{
		Name:   "judge_response",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"score":  map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"reason": map[string]any{"type": "string"},
			},
			"required":             []string{"score", "reason"},
			"additionalProperties": false,
		},
	}
}

func (judgeContract) Decode(raw []byte) (judgeResponse, error) {
	var resp judgeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return judgeResponse{}, fmt.Errorf("%w: unmarshal: %w", ErrJudgeContractNotMet, err)
	}
	if err := resp.validate(); err != nil {
		return judgeResponse{}, fmt.Errorf("%w: %w", ErrJudgeContractNotMet, err)
	}
	return resp, nil
}

type llmJudgedScorer struct {
	id           string
	provider     llm.Provider
	instructions string
	maxTokens    int
}

func NewLLMJudgedScorer(id string, provider llm.Provider, instructions string) Scorer {
	return &llmJudgedScorer{
		id:           id,
		provider:     provider,
		instructions: instructions,
		maxTokens:    512,
	}
}

func (s *llmJudgedScorer) ID() string       { return s.id }
func (s *llmJudgedScorer) Kind() ScorerKind { return ScorerKindLLMJudged }

func (s *llmJudgedScorer) Score(ctx context.Context, sample RunSample) (ScoreResult, error) {
	contract := judgeContract{}
	schema := contract.Schema()

	userContent := fmt.Sprintf(
		"Input:\n%s\n\nExpected Output:\n%s\n\nActual Output:\n%s",
		sample.Input,
		sample.ExpectedOutput,
		sample.Output,
	)

	req := llm.Request{
		Messages: []llm.Message{
			{Role: "system", Content: s.instructions},
			{Role: "user", Content: userContent},
		},
		Schema:    &schema,
		MaxTokens: s.maxTokens,
	}

	resp, err := s.provider.Complete(ctx, req)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("scorer.llm_judged.score: complete: %w", err)
	}

	judgment, err := contract.Decode(resp.RawJSON)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("scorer.llm_judged.score: decode: %w", err)
	}

	return ScoreResult{
		Score:  judgment.Score,
		Reason: judgment.Reason,
		Metadata: map[string]any{
			"prompt_tokens":     resp.PromptTokens,
			"completion_tokens": resp.CompletionTokens,
		},
	}, nil
}
