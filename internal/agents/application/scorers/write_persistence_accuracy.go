package scorers

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

var mecontrolaEffectiveWriteOutcomes = map[string]bool{
	agent.ToolOutcomeRouted.String():     true,
	agent.ToolOutcomeReconciled.String(): true,
}

var mecontrolaNeutralWriteOutcomes = map[string]bool{
	agent.ToolOutcomeReplay.String():  true,
	agent.ToolOutcomeClarify.String(): true,
}

type writePersistenceAccuracyScorer struct{}

func (s *writePersistenceAccuracyScorer) ID() string              { return "write_persistence_accuracy" }
func (s *writePersistenceAccuracyScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *writePersistenceAccuracyScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	writeSet := make(map[string]bool, len(mecontrolaWriteTools))
	for _, t := range mecontrolaWriteTools {
		writeSet[t] = true
	}
	denominator := 0
	numerator := 0
	for _, tc := range sample.ToolCalls {
		if !writeSet[tc.Name] {
			continue
		}
		if mecontrolaNeutralWriteOutcomes[tc.Outcome] {
			continue
		}
		denominator++
		if mecontrolaEffectiveWriteOutcomes[tc.Outcome] {
			numerator++
			continue
		}
		return scorer.ScoreResult{
			Score:    0.0,
			Reason:   fmt.Sprintf("write-tool %s sem efeito de persistência: outcome %q", tc.Name, tc.Outcome),
			Metadata: map[string]any{"tool": tc.Name, "outcome": tc.Outcome},
		}, nil
	}
	if denominator == 0 {
		return scorer.ScoreResult{Score: 1.0, Reason: "nenhuma write-tool efetivada no sample; neutro"}, nil
	}
	return scorer.ScoreResult{
		Score:    1.0,
		Reason:   fmt.Sprintf("%d/%d write-tools com efeito de persistência", numerator, denominator),
		Metadata: map[string]any{"effective": numerator, "total": denominator},
	}, nil
}

func NewWritePersistenceAccuracyScorer() scorer.Scorer { return &writePersistenceAccuracyScorer{} }
