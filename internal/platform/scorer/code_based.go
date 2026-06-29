package scorer

import (
	"context"
	"encoding/json"
	"fmt"
)

type toolCallAccuracyScorer struct {
	id       string
	expected []string
}

func NewToolCallAccuracyScorer(id string, expectedTools []string) Scorer {
	return &toolCallAccuracyScorer{id: id, expected: expectedTools}
}

func (s *toolCallAccuracyScorer) ID() string       { return s.id }
func (s *toolCallAccuracyScorer) Kind() ScorerKind { return ScorerKindCodeBased }

func (s *toolCallAccuracyScorer) Score(_ context.Context, sample RunSample) (ScoreResult, error) {
	if len(s.expected) == 0 {
		return ScoreResult{Score: 1.0, Reason: "no expected tools; trivially accurate"}, nil
	}

	called := make(map[string]bool, len(sample.ToolCalls))
	for _, tc := range sample.ToolCalls {
		called[tc.Name] = true
	}

	matched := 0
	for _, exp := range s.expected {
		if called[exp] {
			matched++
		}
	}

	score := float64(matched) / float64(len(s.expected))
	reason := fmt.Sprintf("matched %d/%d expected tool calls", matched, len(s.expected))

	return ScoreResult{
		Score:  score,
		Reason: reason,
		Metadata: map[string]any{
			"expected": s.expected,
			"matched":  matched,
		},
	}, nil
}

type completenessScorer struct {
	id             string
	requiredFields []string
}

func NewCompletenessScorer(id string, requiredFields []string) Scorer {
	return &completenessScorer{id: id, requiredFields: requiredFields}
}

func (s *completenessScorer) ID() string       { return s.id }
func (s *completenessScorer) Kind() ScorerKind { return ScorerKindCodeBased }

func (s *completenessScorer) Score(_ context.Context, sample RunSample) (ScoreResult, error) {
	if len(s.requiredFields) == 0 {
		return ScoreResult{Score: 1.0, Reason: "no required fields; trivially complete"}, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(sample.Output), &parsed); err != nil {
		return ScoreResult{
			Score:  0,
			Reason: "output is not valid JSON",
			Metadata: map[string]any{
				"parse_error": err.Error(),
			},
		}, nil
	}

	present := 0
	missing := make([]string, 0, len(s.requiredFields))
	for _, field := range s.requiredFields {
		if _, ok := parsed[field]; ok {
			present++
		} else {
			missing = append(missing, field)
		}
	}

	score := float64(present) / float64(len(s.requiredFields))
	reason := fmt.Sprintf("present %d/%d required fields", present, len(s.requiredFields))
	if len(missing) > 0 {
		reason += fmt.Sprintf("; missing: %v", missing)
	}

	return ScoreResult{
		Score:  score,
		Reason: reason,
		Metadata: map[string]any{
			"required": s.requiredFields,
			"missing":  missing,
			"present":  present,
		},
	}, nil
}
