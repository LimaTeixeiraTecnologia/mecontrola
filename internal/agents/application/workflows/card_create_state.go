package workflows

import (
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
