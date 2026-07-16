package workflows

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"

const nearLimitThresholdBP = 9000

func DecideCategorySummaryScenario(plannedCents *int64, spentCents int64) messages.SummaryScenario {
	if plannedCents == nil || *plannedCents <= 0 {
		return messages.SummaryScenarioAvailable
	}
	planned := *plannedCents
	switch {
	case spentCents > planned:
		return messages.SummaryScenarioExceeded
	case spentCents == planned:
		return messages.SummaryScenarioExactLimit
	case spentCents*10000 >= planned*nearLimitThresholdBP:
		return messages.SummaryScenarioNearLimit
	default:
		return messages.SummaryScenarioAvailable
	}
}

func DecideGeneralSummaryScenario(totalPlannedCents *int64, totalSpentCents int64) messages.GeneralScenario {
	if totalPlannedCents == nil || *totalPlannedCents <= 0 {
		return messages.GeneralScenarioPositive
	}
	planned := *totalPlannedCents
	switch {
	case totalSpentCents > planned:
		return messages.GeneralScenarioCritical
	case totalSpentCents*10000 >= planned*nearLimitThresholdBP:
		return messages.GeneralScenarioAttention
	default:
		return messages.GeneralScenarioPositive
	}
}
