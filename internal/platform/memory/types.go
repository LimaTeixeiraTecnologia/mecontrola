package memory

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
	RoleSystem    MessageRole = "system"
)

func (r MessageRole) String() string {
	return string(r)
}

func (r MessageRole) IsValid() bool {
	switch r {
	case RoleUser, RoleAssistant, RoleTool, RoleSystem:
		return true
	}
	return false
}

func ParseMessageRole(s string) (MessageRole, error) {
	r := MessageRole(s)
	if !r.IsValid() {
		return "", fmt.Errorf("invalid message role: %s", s)
	}
	return r, nil
}

type Thread struct {
	ID         uuid.UUID
	ResourceID string
	ThreadID   string
	Title      string
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Message struct {
	ID         uuid.UUID
	ThreadPK   uuid.UUID
	ResourceID string
	Role       MessageRole
	Content    string
	Parts      []byte
	CreatedAt  time.Time
}

type RecallHit struct {
	ResourceID string
	ThreadID   string
	Content    string
	Score      float64
}

type IndexMessagePayload struct {
	ResourceID string    `json:"resource_id"`
	ThreadID   string    `json:"thread_id"`
	MessagePK  uuid.UUID `json:"message_pk"`
	Content    string    `json:"content"`
	Model      string    `json:"model"`
}
