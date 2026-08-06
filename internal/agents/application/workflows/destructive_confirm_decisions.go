package workflows

import (
	"time"
)

const destructiveManageTTL = 5 * time.Minute

type DestructiveManageAction int

const (
	DestructiveManageActionAccept DestructiveManageAction = iota + 1
	DestructiveManageActionCancel
	DestructiveManageActionReprompt
	DestructiveManageActionExpire
)

func isDestructiveManageExpired(state DestructiveManageState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > destructiveManageTTL
}

func DecideDestructiveManageConfirmation(state DestructiveManageState, msg PendingMessage, now time.Time) DestructiveManageAction {
	if isDestructiveManageExpired(state, now) {
		return DestructiveManageActionExpire
	}

	switch DecideConfirmAnswer(msg.Text) {
	case ConfirmAnswerYes:
		return DestructiveManageActionAccept
	case ConfirmAnswerNo:
		return DestructiveManageActionCancel
	}

	if state.RepromptDone {
		return DestructiveManageActionCancel
	}

	return DestructiveManageActionReprompt
}
