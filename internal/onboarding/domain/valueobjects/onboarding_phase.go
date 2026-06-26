package valueobjects

import "errors"

var ErrOnboardingPhaseInvalid = errors.New("onboarding: phase invalid")

type OnboardingPhase uint8

const (
	PhaseWelcome OnboardingPhase = iota + 1
	PhaseObjective
	PhaseBudget
	PhaseCards
	PhaseCategories
	PhaseValues
	PhaseSummary
	PhaseConclusion
)

func (p OnboardingPhase) String() string {
	switch p {
	case PhaseWelcome:
		return "welcome"
	case PhaseObjective:
		return "objective"
	case PhaseBudget:
		return "budget"
	case PhaseCards:
		return "cards"
	case PhaseCategories:
		return "categories"
	case PhaseValues:
		return "values"
	case PhaseSummary:
		return "summary"
	case PhaseConclusion:
		return "conclusion"
	default:
		return "unknown"
	}
}

func (p OnboardingPhase) IsValid() bool {
	return p >= PhaseWelcome && p <= PhaseConclusion
}

func ParseOnboardingPhase(raw string) (OnboardingPhase, error) {
	switch raw {
	case "welcome":
		return PhaseWelcome, nil
	case "objective":
		return PhaseObjective, nil
	case "budget":
		return PhaseBudget, nil
	case "cards":
		return PhaseCards, nil
	case "categories":
		return PhaseCategories, nil
	case "values":
		return PhaseValues, nil
	case "summary":
		return PhaseSummary, nil
	case "conclusion":
		return PhaseConclusion, nil
	default:
		return 0, ErrOnboardingPhaseInvalid
	}
}
