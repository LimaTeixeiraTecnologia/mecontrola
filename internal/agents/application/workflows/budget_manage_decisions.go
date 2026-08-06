package workflows

import (
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	budgetManageTTL          = 30 * time.Minute
	budgetManageMaxReprompts = 1
)

var ErrBudgetManageAcceptedWithoutResource = ErrWriteAcceptedWithoutResource

func DecideBudgetManagePostWrite(resourceID string) (workflow.StepStatus, error) {
	if strings.TrimSpace(resourceID) == "" {
		return workflow.StepStatusFailed, ErrBudgetManageAcceptedWithoutResource
	}
	return workflow.StepStatusCompleted, nil
}

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
	ApplyScope  BudgetManageApplyScope
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

func DecideBudgetManageApplyScope(text string) BudgetManageDecision {
	normalized := normalizeText(text)
	switch {
	case normalized == "":
		return BudgetManageDecision{Action: BudgetManageActionRepromptConfirm}
	case strings.Contains(normalized, "subsequente"),
		strings.Contains(normalized, "seguinte"),
		strings.Contains(normalized, "proximo"),
		strings.Contains(normalized, "todos"),
		strings.Contains(normalized, "todas"):
		return BudgetManageDecision{
			Action:     BudgetManageActionAdvanceToConfirm,
			ApplyScope: BudgetManageApplyScopeCurrentAndSubsequent,
		}
	case strings.Contains(normalized, "vigente"),
		strings.Contains(normalized, "atual"),
		strings.Contains(normalized, "esse mes"),
		strings.Contains(normalized, "este mes"),
		strings.Contains(normalized, "somente"),
		strings.Contains(normalized, "apenas"),
		strings.Contains(normalized, "so"):
		return BudgetManageDecision{
			Action:     BudgetManageActionAdvanceToConfirm,
			ApplyScope: BudgetManageApplyScopeCurrentOnly,
		}
	default:
		return BudgetManageDecision{Action: BudgetManageActionRepromptConfirm}
	}
}

func isBudgetManageConfirmYes(text string) bool {
	return DecideConfirmAnswer(text) == ConfirmAnswerYes
}

func isBudgetManageConfirmNo(text string) bool {
	return DecideConfirmAnswer(text) == ConfirmAnswerNo
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
