package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrThreadIDRequired      = errors.New("agent.thread: id é obrigatório")
	ErrThreadUserRequired    = errors.New("agent.thread: user_id é obrigatório")
	ErrThreadChannelRequired = errors.New("agent.thread: channel é obrigatório")
	ErrThreadChannelTooLong  = errors.New("agent.thread: channel deve ter entre 1 e 32 caracteres")
)

const threadChannelMaxLen = 32

type Thread struct {
	id        uuid.UUID
	userID    uuid.UUID
	channel   string
	createdAt time.Time
	updatedAt time.Time
}

func NewThread(userID uuid.UUID, channel string) (Thread, error) {
	if userID == uuid.Nil {
		return Thread{}, ErrThreadUserRequired
	}
	normalized := strings.TrimSpace(channel)
	if normalized == "" {
		return Thread{}, ErrThreadChannelRequired
	}
	if len(normalized) > threadChannelMaxLen {
		return Thread{}, ErrThreadChannelTooLong
	}
	now := time.Now().UTC()
	return Thread{
		id:        uuid.New(),
		userID:    userID,
		channel:   normalized,
		createdAt: now,
		updatedAt: now,
	}, nil
}

type ThreadParams struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Channel   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func RestoreThread(params ThreadParams) (Thread, error) {
	if params.ID == uuid.Nil {
		return Thread{}, ErrThreadIDRequired
	}
	if params.UserID == uuid.Nil {
		return Thread{}, ErrThreadUserRequired
	}
	normalized := strings.TrimSpace(params.Channel)
	if normalized == "" {
		return Thread{}, ErrThreadChannelRequired
	}
	if len(normalized) > threadChannelMaxLen {
		return Thread{}, ErrThreadChannelTooLong
	}
	return Thread{
		id:        params.ID,
		userID:    params.UserID,
		channel:   normalized,
		createdAt: params.CreatedAt.UTC(),
		updatedAt: params.UpdatedAt.UTC(),
	}, nil
}

func (t Thread) ID() uuid.UUID        { return t.id }
func (t Thread) UserID() uuid.UUID    { return t.userID }
func (t Thread) Channel() string      { return t.channel }
func (t Thread) CreatedAt() time.Time { return t.createdAt }
func (t Thread) UpdatedAt() time.Time { return t.updatedAt }
