package services

import (
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrThresholdPlannedZero = errors.New("budgets: planned_cents deve ser maior que zero para avaliação de limiar")

type Transition struct {
	Threshold        valueobjects.Threshold
	NowCrossed       bool
	WasCrossed       bool
	IsRealTransition bool
}

type ThresholdEvaluator struct{}

func (ThresholdEvaluator) EvaluateThresholds(spentCents int64, plannedCents int64, currentlyCrossed map[valueobjects.Threshold]bool) ([]Transition, error) {
	if plannedCents <= 0 {
		return nil, fmt.Errorf("budgets: planned=%d: %w", plannedCents, ErrThresholdPlannedZero)
	}

	thresholds := [2]valueobjects.Threshold{valueobjects.Threshold80, valueobjects.Threshold100}
	transitions := make([]Transition, 0, 2)
	for _, t := range thresholds {
		nowCrossed := isCrossed(spentCents, plannedCents, t)
		wasCrossed := currentlyCrossed[t]
		transitions = append(transitions, Transition{
			Threshold:        t,
			NowCrossed:       nowCrossed,
			WasCrossed:       wasCrossed,
			IsRealTransition: nowCrossed != wasCrossed,
		})
	}
	return transitions, nil
}

func isCrossed(spent, planned int64, t valueobjects.Threshold) bool {
	threshold := int64(t.Int())
	return spent*100 >= planned*threshold
}
