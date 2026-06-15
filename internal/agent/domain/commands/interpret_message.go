package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var ErrInterpretMessageEmptyText = errors.New("agent.llm: interpret message text is empty")

var ErrInterpretMessageTextTooLong = errors.New("agent.llm: interpret message text exceeds maximum length")

var ErrInterpretMessageChannelEmpty = errors.New("agent.llm: interpret message channel is empty")

var ErrInterpretMessageUserIDRequired = errors.New("agent.llm: interpret message user_id is required")

const maxUserMessageLength = 4096

type RawInterpretMessage struct {
	UserID  uuid.UUID
	Channel string
	Text    string
}

type InterpretMessage struct {
	UserID  uuid.UUID
	Channel string
	Text    string
}

func NewInterpretMessage(raw RawInterpretMessage) (InterpretMessage, error) {
	if raw.UserID == uuid.Nil {
		return InterpretMessage{}, ErrInterpretMessageUserIDRequired
	}
	channel := strings.ToLower(strings.TrimSpace(raw.Channel))
	if channel == "" {
		return InterpretMessage{}, ErrInterpretMessageChannelEmpty
	}
	trimmed := strings.TrimSpace(raw.Text)
	if trimmed == "" {
		return InterpretMessage{}, ErrInterpretMessageEmptyText
	}
	if len([]rune(trimmed)) > maxUserMessageLength {
		return InterpretMessage{}, fmt.Errorf("agent.llm: %w (max %d runes)", ErrInterpretMessageTextTooLong, maxUserMessageLength)
	}
	return InterpretMessage{
		UserID:  raw.UserID,
		Channel: channel,
		Text:    trimmed,
	}, nil
}
