package workflows

import (
	"strings"
	"time"
)

const (
	budgetManageTTL          = 30 * time.Minute
	budgetManageMaxReprompts = 1
)

type BudgetManageAction int

const (
	BudgetManageActionNone BudgetManageAction = iota + 1
	BudgetManageActionFillTotal
	BudgetManageActionRepromptTotal
	BudgetManageActionRepromptDistribution
	BudgetManageActionAdvanceToConfirm
	BudgetManageActionConfirm
	BudgetManageActionCancel
	BudgetManageActionRepromptConfirm
	BudgetManageActionExpire
	BudgetManageActionReplay
)

type BudgetManageDecision struct {
	Action      BudgetManageAction
	TotalCents  int64
	Allocations map[string]int
}

type BudgetManageMessage struct {
	Text      string
	MessageID string
}

func isBudgetManageExpired(state BudgetManageState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > budgetManageTTL
}

func DecideBudgetManageTotal(totalCents int64) BudgetManageDecision {
	if totalCents <= 0 {
		return BudgetManageDecision{Action: BudgetManageActionRepromptTotal}
	}
	return BudgetManageDecision{Action: BudgetManageActionFillTotal, TotalCents: totalCents}
}

func DecideBudgetManageDistribution(allocations map[string]int) BudgetManageDecision {
	total := 0
	for _, bp := range allocations {
		total += bp
	}
	if total != 10000 {
		return BudgetManageDecision{Action: BudgetManageActionRepromptDistribution, Allocations: allocations}
	}
	return BudgetManageDecision{Action: BudgetManageActionAdvanceToConfirm, Allocations: allocations}
}

func isBudgetManageConfirmYes(text string) bool {
	return reConfirmYes.MatchString(strings.TrimSpace(text))
}

func isBudgetManageConfirmNo(text string) bool {
	normalized := strings.TrimSpace(text)
	return reConfirmNo.MatchString(normalized) || isCancelMessage(normalized)
}

func DecideBudgetManageConfirmation(state BudgetManageState, msg BudgetManageMessage, now time.Time) BudgetManageDecision {
	if isBudgetManageExpired(state, now) {
		return BudgetManageDecision{Action: BudgetManageActionExpire}
	}

	if msg.MessageID != "" && msg.MessageID == state.MessageID {
		return BudgetManageDecision{Action: BudgetManageActionReplay}
	}

	text := strings.TrimSpace(msg.Text)

	if isBudgetManageConfirmYes(text) {
		return BudgetManageDecision{Action: BudgetManageActionConfirm}
	}

	if isBudgetManageConfirmNo(text) {
		return BudgetManageDecision{Action: BudgetManageActionCancel}
	}

	if state.RepromptCount >= budgetManageMaxReprompts {
		return BudgetManageDecision{Action: BudgetManageActionCancel}
	}

	return BudgetManageDecision{Action: BudgetManageActionRepromptConfirm}
}
