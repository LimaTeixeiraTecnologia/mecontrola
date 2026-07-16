package workflows

import (
	"strings"
	"time"
)

const (
	cardManageConfirmTTL   = 15 * time.Minute
	cardManageMaxReprompts = 1
)

type CardManageAction int

const (
	CardManageActionAccept CardManageAction = iota + 1
	CardManageActionCancel
	CardManageActionReprompt
	CardManageActionExpire
	CardManageActionReplay
)

func isCardManageExpired(state CardManageState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > cardManageConfirmTTL
}

func DecideCardManageConfirmation(state CardManageState, msg PendingMessage, now time.Time) CardManageAction {
	if isCardManageExpired(state, now) {
		return CardManageActionExpire
	}

	if msg.MessageID != "" && msg.MessageID == state.ProcessedMessageID {
		return CardManageActionReplay
	}

	text := strings.TrimSpace(msg.Text)

	if reConfirmYes.MatchString(text) {
		return CardManageActionAccept
	}

	if reConfirmNo.MatchString(text) || isCancelMessage(text) {
		return CardManageActionCancel
	}

	if state.ConfirmReprompt >= cardManageMaxReprompts {
		return CardManageActionCancel
	}

	return CardManageActionReprompt
}
