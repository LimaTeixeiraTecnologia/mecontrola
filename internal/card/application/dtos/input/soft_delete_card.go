package input

import "github.com/google/uuid"

type SoftDeleteCard struct {
	ID     uuid.UUID
	UserID uuid.UUID
}
