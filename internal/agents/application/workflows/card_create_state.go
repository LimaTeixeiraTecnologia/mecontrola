package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CardCreateStatus int

const (
	CardCreateStatusActive CardCreateStatus = iota + 1
	CardCreateStatusCompleted
	CardCreateStatusCancelled
	CardCreateStatusExpired
)

func (s CardCreateStatus) String() string {
	switch s {
	case CardCreateStatusActive:
		return "active"
	case CardCreateStatusCompleted:
		return "completed"
	case CardCreateStatusCancelled:
		return "cancelled"
	case CardCreateStatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (s CardCreateStatus) IsValid() bool {
	return s >= CardCreateStatusActive && s <= CardCreateStatusExpired
}

var errInvalidCardCreateStatus = errors.New("workflows: card create status inválido")

func ParseCardCreateStatus(v string) (CardCreateStatus, error) {
	switch v {
	case "active":
		return CardCreateStatusActive, nil
	case "completed":
		return CardCreateStatusCompleted, nil
	case "cancelled":
		return CardCreateStatusCancelled, nil
	case "expired":
		return CardCreateStatusExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidCardCreateStatus, v)
	}
}

type CardCreateState struct {
	Status             CardCreateStatus `json:"status"`
	Awaiting           AwaitingKind     `json:"awaiting"`
	UserID             uuid.UUID        `json:"userId"`
	Nickname           string           `json:"nickname"`
	Bank               string           `json:"bank"`
	DueDay             int              `json:"dueDay"`
	ClosingDay         int              `json:"closingDay"`
	ClosingDayProvided bool             `json:"closingDayProvided"`
	MessageID          string           `json:"messageId"`
	IncomingMessageID  string           `json:"incomingMessageId"`
	ProcessedMessageID string           `json:"processedMessageId"`
	ConfirmReprompt    int              `json:"confirmReprompt"`
	SuspendedAt        time.Time        `json:"suspendedAt"`
	ResumeText         string           `json:"resumeText"`
	ResponseText       string           `json:"responseText"`
	Expired            bool             `json:"expired"`
}
