package workflows

import (
	"strings"
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

	text := strings.ToLower(strings.TrimSpace(msg.Text))

	if isSim(text) {
		return DestructiveManageActionAccept
	}

	if isNao(text) {
		return DestructiveManageActionCancel
	}

	if state.RepromptDone {
		return DestructiveManageActionCancel
	}

	return DestructiveManageActionReprompt
}
