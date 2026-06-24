package onboardingv2draft

import (
	"encoding/json"
	"fmt"
	"strings"
)

type OnboardingV2Step uint8

const (
	StepObjective OnboardingV2Step = iota + 1
	StepBudget
	StepCards
	StepFinancialPlan
)

const PendingActionKind = "onboarding_v2"

func (s OnboardingV2Step) String() string {
	switch s {
	case StepObjective:
		return "objective"
	case StepBudget:
		return "budget"
	case StepCards:
		return "cards"
	case StepFinancialPlan:
		return "financial_plan"
	default:
		return fmt.Sprintf("unknown_step_%d", uint8(s))
	}
}

func ParseOnboardingV2Step(raw string) (OnboardingV2Step, error) {
	switch raw {
	case "objective":
		return StepObjective, nil
	case "budget":
		return StepBudget, nil
	case "cards":
		return StepCards, nil
	case "financial_plan":
		return StepFinancialPlan, nil
	default:
		return 0, fmt.Errorf("onboardingv2draft: invalid step %q", raw)
	}
}

type CardEntry struct {
	Name   string `json:"name"`
	DueDay int    `json:"due_day"`
}

type SplitEntry struct {
	RootSlug    string `json:"root_slug"`
	AmountCents int64  `json:"amount_cents"`
}

type Draft struct {
	step               OnboardingV2Step
	objective          string
	objectiveProfile   string
	incomeCents        int64
	cards              []CardEntry
	splits             []SplitEntry
	autoSplitGenerated bool
}

type draftJSON struct {
	Kind               string       `json:"kind"`
	Step               uint8        `json:"step"`
	Objective          string       `json:"objective,omitempty"`
	ObjectiveProfile   string       `json:"objective_profile,omitempty"`
	IncomeCents        int64        `json:"income_cents,omitempty"`
	Cards              []CardEntry  `json:"cards,omitempty"`
	Splits             []SplitEntry `json:"splits,omitempty"`
	AutoSplitGenerated bool         `json:"auto_split_generated,omitempty"`
}

func New() Draft {
	return Draft{step: StepObjective}
}

func Encode(d Draft) ([]byte, error) {
	return json.Marshal(draftJSON{
		Kind:               PendingActionKind,
		Step:               uint8(d.step),
		Objective:          d.objective,
		ObjectiveProfile:   d.objectiveProfile,
		IncomeCents:        d.incomeCents,
		Cards:              d.cards,
		Splits:             d.splits,
		AutoSplitGenerated: d.autoSplitGenerated,
	})
}

func Restore(raw []byte) (Draft, error) {
	var j draftJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return Draft{}, fmt.Errorf("onboardingv2draft: unmarshal: %w", err)
	}
	if j.Kind != PendingActionKind {
		return Draft{}, fmt.Errorf("onboardingv2draft: unexpected kind %q", j.Kind)
	}
	step := OnboardingV2Step(j.Step)
	if step.String() == fmt.Sprintf("unknown_step_%d", j.Step) {
		return Draft{}, fmt.Errorf("onboardingv2draft: invalid step %d", j.Step)
	}
	return Draft{
		step:               step,
		objective:          j.Objective,
		objectiveProfile:   j.ObjectiveProfile,
		incomeCents:        j.IncomeCents,
		cards:              j.Cards,
		splits:             j.Splits,
		autoSplitGenerated: j.AutoSplitGenerated,
	}, nil
}

func IsDraftPending(raw []byte) bool {
	var j struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &j); err != nil {
		return false
	}
	return j.Kind == PendingActionKind
}

func (d Draft) Step() OnboardingV2Step   { return d.step }
func (d Draft) Objective() string        { return d.objective }
func (d Draft) ObjectiveProfile() string { return d.objectiveProfile }
func (d Draft) IncomeCents() int64       { return d.incomeCents }
func (d Draft) Cards() []CardEntry       { return d.cards }
func (d Draft) Splits() []SplitEntry     { return d.splits }
func (d Draft) HasAutoSplits() bool      { return d.autoSplitGenerated }

func (d Draft) WithStep(s OnboardingV2Step) Draft {
	d.step = s
	return d
}

func (d Draft) WithObjective(obj string) Draft {
	d.objective = obj
	return d
}

func (d Draft) WithObjectiveProfile(profile string) Draft {
	d.objectiveProfile = profile
	return d
}

func (d Draft) WithIncome(cents int64) Draft {
	d.incomeCents = cents
	return d
}

func (d Draft) WithAutoSplits(splits []SplitEntry) Draft {
	d.splits = splits
	d.autoSplitGenerated = true
	return d
}

func (d Draft) WithAdjustedSplits(splits []SplitEntry) Draft {
	d.splits = splits
	return d
}

func (d Draft) AppendCard(c CardEntry) Draft {
	for _, existing := range d.cards {
		if strings.EqualFold(existing.Name, c.Name) {
			return d
		}
	}
	d.cards = append(d.cards, c)
	return d
}
