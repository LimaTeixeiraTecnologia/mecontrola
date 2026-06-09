package input

import "github.com/google/uuid"

type CreateCard struct {
	UserID     uuid.UUID
	Name       string
	Nickname   string
	ClosingDay int
	DueDay     int
}
