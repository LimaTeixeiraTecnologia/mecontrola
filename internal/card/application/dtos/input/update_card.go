package input

import "github.com/google/uuid"

type UpdateCard struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	Nickname   string
	ClosingDay int
	DueDay     int
}
