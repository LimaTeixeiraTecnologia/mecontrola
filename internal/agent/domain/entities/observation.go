package entities

import (
	"time"

	"github.com/google/uuid"
)

type Observation struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Channel   string
	Content   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewObservation(userID uuid.UUID, channel, content string, now time.Time) Observation {
	return Observation{
		ID:        uuid.New(),
		UserID:    userID,
		Channel:   channel,
		Content:   content,
		CreatedAt: now,
		ExpiresAt: now.AddDate(0, 0, 90),
	}
}
