package intent

import "errors"

type PlanKind int

const (
	PlanKindSingle PlanKind = iota + 1
	PlanKindMulti
)

func (k PlanKind) String() string {
	switch k {
	case PlanKindSingle:
		return "single"
	case PlanKindMulti:
		return "multi"
	default:
		return "unknown"
	}
}

var ErrPlanEmpty = errors.New("agent.intent: plan must have at least one step")

type IntentStep struct {
	Intent     Intent
	Confidence float64
	Index      int
}

type IntentPlan struct {
	Steps []IntentStep
	kind  PlanKind
}

func NewIntentPlan(steps []IntentStep) (IntentPlan, error) {
	if len(steps) == 0 {
		return IntentPlan{}, ErrPlanEmpty
	}
	k := PlanKindSingle
	if len(steps) > 1 {
		k = PlanKindMulti
	}
	return IntentPlan{Steps: steps, kind: k}, nil
}

func (p IntentPlan) Kind() PlanKind { return p.kind }
func (p IntentPlan) Len() int       { return len(p.Steps) }
func (p IntentPlan) IsSingle() bool { return p.kind == PlanKindSingle }
func (p IntentPlan) HasWrite() bool {
	for _, s := range p.Steps {
		if s.Intent.Kind().IsWrite() {
			return true
		}
	}
	return false
}
