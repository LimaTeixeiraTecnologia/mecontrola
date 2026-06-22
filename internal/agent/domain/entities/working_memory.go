package entities

import (
	"time"

	"github.com/google/uuid"
)

type WorkingMemory struct {
	UserID    uuid.UUID
	Content   string
	UpdatedAt time.Time
}

func NewWorkingMemory(userID uuid.UUID) WorkingMemory {
	return WorkingMemory{UserID: userID}
}

func (w *WorkingMemory) Update(content string, now time.Time) {
	w.Content = content
	w.UpdatedAt = now
}
