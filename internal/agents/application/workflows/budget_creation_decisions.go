package workflows

import (
	"strings"
	"time"
)

const (
	budgetCreationTTL          = 30 * time.Minute
	budgetCreationMaxReprompts = 1
)

type BudgetCreationAction int

const (
	BudgetActionNone BudgetCreationAction = iota + 1
	BudgetActionFillTotal
	BudgetActionRepromptTotal
	BudgetActionRepromptDistribution
	BudgetActionAdvanceToConfirm
	BudgetActionConfirm
	BudgetActionCancel
	BudgetActionRepromptConfirm
	BudgetActionExpire
	BudgetActionReplay
)

type BudgetCreationDecision struct {
	Action      BudgetCreationAction
	TotalCents  int64
	Allocations map[string]int
}

type BudgetCreationMessage struct {
	Text      string
	MessageID string
}

func isBudgetExpired(state BudgetCreationState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > budgetCreationTTL
}

func DecideBudgetTotal(totalCents int64) BudgetCreationDecision {
	if totalCents <= 0 {
		return BudgetCreationDecision{Action: BudgetActionRepromptTotal}
	}
	return BudgetCreationDecision{Action: BudgetActionFillTotal, TotalCents: totalCents}
}

func DecideBudgetDistribution(allocations map[string]int) BudgetCreationDecision {
	total := 0
	for _, bp := range allocations {
		total += bp
	}
	if total != 10000 {
		return BudgetCreationDecision{Action: BudgetActionRepromptDistribution, Allocations: allocations}
	}
	return BudgetCreationDecision{Action: BudgetActionAdvanceToConfirm, Allocations: allocations}
}

func DecideBudgetPendingResume(state BudgetCreationState, msg BudgetCreationMessage, now time.Time) (BudgetCreationDecision, error) {
	if isBudgetExpired(state, now) {
		return BudgetCreationDecision{Action: BudgetActionExpire}, nil
	}

	if msg.MessageID != "" && msg.MessageID == state.MessageID {
		return BudgetCreationDecision{Action: BudgetActionReplay}, nil
	}

	if state.RepromptCount >= budgetCreationMaxReprompts {
		return BudgetCreationDecision{Action: BudgetActionCancel}, nil
	}

	return BudgetCreationDecision{Action: BudgetActionNone}, nil
}

func isBudgetConfirmYes(text string) bool {
	return reConfirmYes.MatchString(strings.TrimSpace(text))
}

func isBudgetConfirmNo(text string) bool {
	normalized := strings.TrimSpace(text)
	return reConfirmNo.MatchString(normalized) || isCancelMessage(normalized)
}

func DecideBudgetConfirmation(state BudgetCreationState, msg BudgetCreationMessage, now time.Time) BudgetCreationDecision {
	if isBudgetExpired(state, now) {
		return BudgetCreationDecision{Action: BudgetActionExpire}
	}

	if msg.MessageID != "" && msg.MessageID == state.MessageID {
		return BudgetCreationDecision{Action: BudgetActionReplay}
	}

	text := strings.TrimSpace(msg.Text)

	if isBudgetConfirmYes(text) {
		return BudgetCreationDecision{Action: BudgetActionConfirm}
	}

	if isBudgetConfirmNo(text) {
		return BudgetCreationDecision{Action: BudgetActionCancel}
	}

	if state.RepromptCount >= budgetCreationMaxReprompts {
		return BudgetCreationDecision{Action: BudgetActionCancel}
	}

	return BudgetCreationDecision{Action: BudgetActionRepromptConfirm}
}
