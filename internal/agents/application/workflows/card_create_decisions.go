package workflows

import (
	"strings"
	"time"
)

const (
	cardCreateConfirmTTL   = 15 * time.Minute
	cardCreateMaxReprompts = 1
)

type CardConfirmAction int

const (
	CardConfirmAccept CardConfirmAction = iota + 1
	CardConfirmCancel
	CardConfirmReprompt
	CardConfirmExpire
	CardConfirmReplay
)

func isCardCreateExpired(state CardCreateState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > cardCreateConfirmTTL
}

func DecideCardCreateConfirmation(state CardCreateState, msg PendingMessage, now time.Time) CardConfirmAction {
	if isCardCreateExpired(state, now) {
		return CardConfirmExpire
	}

	if msg.MessageID != "" && msg.MessageID == state.ProcessedMessageID {
		return CardConfirmReplay
	}

	text := strings.TrimSpace(msg.Text)

	if reConfirmYes.MatchString(text) {
		return CardConfirmAccept
	}

	if reConfirmNo.MatchString(text) || isCancelMessage(text) {
		return CardConfirmCancel
	}

	if state.ConfirmReprompt >= cardCreateMaxReprompts {
		return CardConfirmCancel
	}

	return CardConfirmReprompt
}
