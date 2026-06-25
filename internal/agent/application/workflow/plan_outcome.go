package workflow

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

type planStepDisposition int

const (
	planStepAdvance planStepDisposition = iota + 1
	planStepSuspend
	planStepShortCircuit
)

func planStepDispositionFor(outcome tools.ToolOutcome) planStepDisposition {
	switch outcome {
	case tools.OutcomeRouted, tools.OutcomeReplay, tools.OutcomeFallback:
		return planStepAdvance
	case tools.OutcomeClarify:
		return planStepSuspend
	case tools.OutcomeUsecaseError,
		tools.OutcomePolicyBlocked,
		tools.OutcomeAuthzDenied,
		tools.OutcomeMissingResolver,
		tools.OutcomeParseError,
		tools.OutcomeReplyFailed,
		tools.OutcomeEmptyText:
		return planStepShortCircuit
	default:
		return planStepShortCircuit
	}
}
