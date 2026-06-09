package input

import "github.com/google/uuid"

type ListCards struct {
	UserID uuid.UUID
	Cursor string
	Limit  int
}
