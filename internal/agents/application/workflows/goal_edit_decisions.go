package workflows

import (
	"strings"
	"time"
)

const (
	goalEditTTL          = 15 * time.Minute
	goalEditMaxReprompts = 1
)

type GoalEditAction int

const (
	GoalEditActionAccept GoalEditAction = iota + 1
	GoalEditActionCancel
	GoalEditActionReprompt
	GoalEditActionExpire
	GoalEditActionReplay
)

func isGoalEditExpired(state GoalEditState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > goalEditTTL
}

func DecideGoalEditConfirmation(state GoalEditState, msg PendingMessage, now time.Time) GoalEditAction {
	if isGoalEditExpired(state, now) {
		return GoalEditActionExpire
	}

	if msg.MessageID != "" && msg.MessageID == state.MessageID {
		return GoalEditActionReplay
	}

	text := strings.TrimSpace(msg.Text)

	if reConfirmYes.MatchString(text) {
		return GoalEditActionAccept
	}

	if reConfirmNo.MatchString(text) || isCancelMessage(text) {
		return GoalEditActionCancel
	}

	if state.RepromptCount >= goalEditMaxReprompts {
		return GoalEditActionCancel
	}

	return GoalEditActionReprompt
}

func DecideGoalEditNewGoal(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}
