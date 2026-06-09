package input

import "github.com/google/uuid"

type GetCard struct {
	ID     uuid.UUID
	UserID uuid.UUID
}
