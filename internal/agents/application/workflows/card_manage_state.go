package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CardManageOperationKind int

const (
	CardManageOpCreate CardManageOperationKind = iota + 1
	CardManageOpEdit
)

func (o CardManageOperationKind) String() string {
	switch o {
	case CardManageOpCreate:
		return "create_card"
	case CardManageOpEdit:
		return "edit_card"
	default:
		return "unknown"
	}
}

func (o CardManageOperationKind) IsValid() bool {
	return o >= CardManageOpCreate && o <= CardManageOpEdit
}

var errInvalidCardManageOperationKind = errors.New("workflows: card manage operation kind inválido")

func ParseCardManageOperationKind(s string) (CardManageOperationKind, error) {
	switch s {
	case "create_card":
		return CardManageOpCreate, nil
	case "edit_card":
		return CardManageOpEdit, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidCardManageOperationKind, s)
	}
}

type CardManageStatus int

const (
	CardManageActive CardManageStatus = iota + 1
	CardManageCompleted
	CardManageCancelled
	CardManageExpired
)

func (s CardManageStatus) String() string {
	switch s {
	case CardManageActive:
		return "active"
	case CardManageCompleted:
		return "completed"
	case CardManageCancelled:
		return "cancelled"
	case CardManageExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (s CardManageStatus) IsValid() bool {
	return s >= CardManageActive && s <= CardManageExpired
}

var errInvalidCardManageStatus = errors.New("workflows: card manage status inválido")

func ParseCardManageStatus(s string) (CardManageStatus, error) {
	switch s {
	case "active":
		return CardManageActive, nil
	case "completed":
		return CardManageCompleted, nil
	case "cancelled":
		return CardManageCancelled, nil
	case "expired":
		return CardManageExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidCardManageStatus, s)
	}
}

type CardManageState struct {
	Status             CardManageStatus        `json:"status"`
	Operation          CardManageOperationKind `json:"operation"`
	UserID             uuid.UUID               `json:"userId"`
	CardID             string                  `json:"cardId"`
	Nickname           string                  `json:"nickname"`
	NicknameProvided   bool                    `json:"nicknameProvided"`
	Bank               string                  `json:"bank"`
	BankProvided       bool                    `json:"bankProvided"`
	DueDay             int                     `json:"dueDay"`
	DueDayProvided     bool                    `json:"dueDayProvided"`
	ClosingDay         int                     `json:"closingDay"`
	ClosingDayProvided bool                    `json:"closingDayProvided"`
	PreviousFetched    bool                    `json:"previousFetched"`
	PreviousNickname   string                  `json:"previousNickname"`
	PreviousBank       string                  `json:"previousBank"`
	PreviousDueDay     int                     `json:"previousDueDay"`
	MessageID          string                  `json:"messageId"`
	IncomingMessageID  string                  `json:"incomingMessageId"`
	ProcessedMessageID string                  `json:"processedMessageId"`
	ConfirmReprompt    int                     `json:"confirmReprompt"`
	SuspendedAt        time.Time               `json:"suspendedAt"`
	ResumeText         string                  `json:"resumeText"`
	ResponseText       string                  `json:"responseText"`
	Expired            bool                    `json:"expired"`
}
