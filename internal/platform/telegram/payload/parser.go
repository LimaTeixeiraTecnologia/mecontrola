package payload

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const chatTypePrivate = "private"

type ParseOutcome struct {
	Message  Message
	Kind     RejectionKind
	UpdateID int64
}

func ExtractFirstMessage(raw []byte) (ParseOutcome, error) {
	var u update
	if err := json.Unmarshal(raw, &u); err != nil {
		return ParseOutcome{Kind: RejectInvalidJSON}, fmt.Errorf("telegram.payload: unmarshal: %w", err)
	}
	if u.Message == nil {
		return ParseOutcome{Kind: RejectNoMessage, UpdateID: u.UpdateID}, nil
	}
	msg := *u.Message
	if msg.From == nil || msg.From.ID == 0 {
		return ParseOutcome{Kind: RejectMissingFrom, UpdateID: u.UpdateID}, nil
	}
	if msg.From.IsBot {
		return ParseOutcome{Kind: RejectBotSender, UpdateID: u.UpdateID}, nil
	}
	if strings.TrimSpace(msg.Text) == "" {
		return ParseOutcome{Kind: RejectMissingText, UpdateID: u.UpdateID}, nil
	}
	if !strings.EqualFold(msg.Chat.Type, chatTypePrivate) {
		return ParseOutcome{Kind: RejectNonPrivateChat, UpdateID: u.UpdateID}, nil
	}
	if msg.Date <= 0 {
		return ParseOutcome{Kind: RejectMissingDate, UpdateID: u.UpdateID}, nil
	}

	return ParseOutcome{
		Kind:     RejectAccepted,
		UpdateID: u.UpdateID,
		Message: Message{
			UpdateID:   u.UpdateID,
			MessageID:  msg.MessageID,
			FromUserID: msg.From.ID,
			ChatID:     msg.Chat.ID,
			Text:       msg.Text,
			UnixDate:   msg.Date,
		},
	}, nil
}

func formatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}
