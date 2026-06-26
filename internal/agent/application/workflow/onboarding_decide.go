package workflow

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"

type DecisionOutcome int

const (
	OutcomeAdvance DecisionOutcome = iota + 1
	OutcomeClarify
	OutcomeDeferred
	OutcomeConfirm
	OutcomeCancel
	OutcomeCorrect
	OutcomeReprompt
)

func (o DecisionOutcome) String() string {
	switch o {
	case OutcomeAdvance:
		return "advance"
	case OutcomeClarify:
		return "clarify"
	case OutcomeDeferred:
		return "deferred"
	case OutcomeConfirm:
		return "confirm"
	case OutcomeCancel:
		return "cancel"
	case OutcomeCorrect:
		return "correct"
	case OutcomeReprompt:
		return "reprompt"
	default:
		return "unknown"
	}
}

func (o DecisionOutcome) IsValid() bool {
	return o >= OutcomeAdvance && o <= OutcomeReprompt
}

type ParsedObjective struct {
	Objective    string
	DailyCommand bool
	Ambiguous    bool
}

type ParsedBudget struct {
	IncomeCents  int64
	DailyCommand bool
	Ambiguous    bool
}

type ParsedCards struct {
	DailyCommand bool
	Ambiguous    bool
	Skip         bool
	Nickname     string
	DueDay       int
	AddAnother   bool
}

type ParsedSummary struct {
	Confirm      bool
	Cancel       bool
	Correct      bool
	Target       CorrectionTarget
	NewValue     string
	Ambiguous    bool
	DailyCommand bool
}

type ValuesState struct {
	Values      map[string]int64
	IncomeCents int64
}

func DecideObjective(parsed ParsedObjective) DecisionOutcome {
	if parsed.DailyCommand {
		return OutcomeDeferred
	}
	if parsed.Ambiguous || parsed.Objective == "" {
		return OutcomeClarify
	}
	return OutcomeAdvance
}

func DecideBudget(parsed ParsedBudget) DecisionOutcome {
	if parsed.DailyCommand {
		return OutcomeDeferred
	}
	if parsed.Ambiguous || parsed.IncomeCents <= 0 {
		return OutcomeClarify
	}
	return OutcomeAdvance
}

func DecideCards(parsed ParsedCards) DecisionOutcome {
	if parsed.DailyCommand {
		return OutcomeDeferred
	}
	if parsed.Ambiguous {
		return OutcomeClarify
	}
	if parsed.Skip {
		return OutcomeAdvance
	}
	if parsed.AddAnother {
		return OutcomeAdvance
	}
	if parsed.Nickname == "" || parsed.DueDay < 1 || parsed.DueDay > 31 {
		return OutcomeClarify
	}
	return OutcomeAdvance
}

func DecideValues(state ValuesState) DecisionOutcome {
	if len(state.Values) != 5 {
		return OutcomeClarify
	}
	var sum int64
	for _, slug := range categoryOrder {
		amount, ok := state.Values[slug]
		if !ok {
			return OutcomeClarify
		}
		if amount < 0 {
			return OutcomeClarify
		}
		sum += amount
	}
	if sum != state.IncomeCents {
		return OutcomeClarify
	}
	return OutcomeAdvance
}

func DecideSummary(parsed ParsedSummary) DecisionOutcome {
	if parsed.DailyCommand {
		return OutcomeDeferred
	}
	if parsed.Ambiguous {
		return OutcomeClarify
	}
	if parsed.Cancel {
		return OutcomeCancel
	}
	if parsed.Correct {
		if parsed.Target == CorrectionTargetNone || !parsed.Target.IsValid() {
			return OutcomeClarify
		}
		return OutcomeCorrect
	}
	if parsed.Confirm {
		return OutcomeConfirm
	}
	return OutcomeReprompt
}

func BuildValuesState(values map[string]int64, incomeCents int64) ValuesState {
	copied := make(map[string]int64, len(values))
	for k, v := range values {
		copied[k] = v
	}
	return ValuesState{Values: copied, IncomeCents: incomeCents}
}

func CategorySlug(kind valueobjects.CategoryKind) string {
	return kind.String()
}
